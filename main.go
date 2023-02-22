package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/pojntfx/tapisk/pkg/protocol"
)

var (
	errOldstyleNegotiationUnsupported = errors.New("oldstyle negotiation is unsupported")
	errNoZeroesUnsupported            = errors.New("no zeroes is unsupported")
)

func main() {
	file := flag.String("file", "tapisk.img", "Path to file to expose")
	laddr := flag.String("laddr", fmt.Sprintf(":%v", protocol.NbdDefaultPort), "Listen address")

	flag.Parse()

	f, err := os.OpenFile(*file, os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	l, err := net.Listen("tcp", *laddr)
	if err != nil {
		panic(err)
	}
	defer l.Close()

	log.Println("Listening on", l.Addr())

	clients := 0
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Could not accept connection, continuing:", err)

			continue
		}

		clients++

		log.Printf("%v clients connected", clients)

		go func() {
			defer func() {
				_ = conn.Close()

				clients--

				if err := recover(); err != nil {
					log.Printf("Client disconnected with error: %v", err)
				}

				log.Printf("%v clients connected", clients)
			}()

			// Server handshake
			if err := binary.Write(conn, binary.BigEndian, protocol.NbdNewstyleNegotiation{
				Magic:          protocol.NbdMagic,
				NewstyleMagic:  protocol.NbdNewstyleMagic,
				HandshakeFlags: protocol.NbdFlagFixedNewstyle,
			}); err != nil {
				panic(err)
			}

			// Client handshake
			var clientFlags protocol.NbdClientFlags
			if err := binary.Read(conn, binary.BigEndian, &clientFlags); err != nil {
				panic(err)
			}

			if clientFlags.ClientFlags&protocol.NbdClientFlagFixedNewstyle == 0 {
				panic(errOldstyleNegotiationUnsupported)
			}

			if clientFlags.ClientFlags&protocol.NbdClientFlagNoZeroes != 0 {
				panic(errNoZeroesUnsupported)
			}
		}()
	}
}
