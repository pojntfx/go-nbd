package main

import (
	"encoding/binary"
	"flag"
	"log"
	"net"
	"os"
)

// See https://github.com/freqlabs/nbd-client/blob/master/nbd-protocol.h#L40
const (
	// Negotiation handshake
	NewstyleMagic     = uint64(0x49484156454F5054)
	FlagFixedNewstyle = uint16(1 << 0)
)

func main() {
	file := flag.String("file", "tapisk.img", "Path to file to expose")
	laddr := flag.String("laddr", ":10809", "Listen address")

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

			// Negotiation handshake
			if err := binary.Write(conn, binary.BigEndian, NewstyleMagic); err != nil {
				panic(err)
			}

			if err := binary.Write(conn, binary.BigEndian, FlagFixedNewstyle); err != nil {
				panic(err)
			}
		}()
	}
}
