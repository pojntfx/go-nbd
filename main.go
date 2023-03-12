package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/pojntfx/tapisk/pkg/protocol"
)

func main() {
	file := flag.String("file", "tapisk.img", "Path to file to expose")
	laddr := flag.String("laddr", fmt.Sprintf(":%v", 10809), "Listen address")

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

			// Negotiation
			if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationNewstyleHeader{
				OldstyleMagic:  protocol.NegotiationOptionMagic,
				OptionMagic:    protocol.NegotiationOptionMagic,
				HandshakeFlags: protocol.NegotiationFlagFixedNewstyle,
			}); err != nil {
				panic(err)
			}

			var clientFlags protocol.NegotiationClientFlags
			if err := binary.Read(conn, binary.BigEndian, &clientFlags); err != nil {
				panic(err)
			}

			for {
				var optionHeader protocol.NegotiationOptionHeader
				if err := binary.Read(conn, binary.BigEndian, &optionHeader); err != nil {
					panic(err)
				}

				switch optionHeader.ID {
				case protocol.NegotiationOptionInfo, protocol.NegotiationOptionGo:
					var exportNameLength uint32
					if err := binary.Read(conn, binary.BigEndian, &exportNameLength); err != nil {
						panic(err)
					}

					exportName := make([]byte, exportNameLength)
					if _, err := io.ReadFull(conn, exportName); err != nil {
						panic(err)
					}
				}
			}
		}()
	}
}
