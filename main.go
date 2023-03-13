package main

import (
	"bytes"
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

	stat, err := f.Stat()
	if err != nil {
		panic(err)
	}

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
				OldstyleMagic:  protocol.NEGOTIATION_MAGIC_OLDSTYLE,
				OptionMagic:    protocol.NEGOTIATION_MAGIC_OPTION,
				HandshakeFlags: protocol.NEGOTIATION_HANDSHAKE_FLAG_FIXED_NEWSTYLE,
			}); err != nil {
				panic(err)
			}

			_, err := io.CopyN(io.Discard, conn, 4) // Discard client flags (uint32)
			if err != nil {
				panic(err)
			}

			for {
				var optionHeader protocol.NegotiationOptionHeader
				if err := binary.Read(conn, binary.BigEndian, &optionHeader); err != nil {
					panic(err)
				}

				switch optionHeader.ID {
				case protocol.NEGOTIATION_OPTION_INFO, protocol.NEGOTIATION_OPTION_GO:
					var exportNameLength uint32
					if err := binary.Read(conn, binary.BigEndian, &exportNameLength); err != nil {
						panic(err)
					}

					exportName := make([]byte, exportNameLength)
					if _, err := io.ReadFull(conn, exportName); err != nil {
						panic(err)
					}

					var informationRequestCount uint16
					if err := binary.Read(conn, binary.BigEndian, &informationRequestCount); err != nil {
						panic(err)
					}

					_, err := io.CopyN(io.Discard, conn, 2*int64(informationRequestCount)) // Discard information requests (uint16s)
					if err != nil {
						panic(err)
					}

					replyInfo := &bytes.Buffer{}
					if err := binary.Write(replyInfo, binary.BigEndian, protocol.NegotiationReplyInfo{
						Type:              protocol.NEGOTIATION_TYPE_INFO_EXPORT,
						Size:              uint64(stat.Size()),
						TransmissionFlags: 0,
					}); err != nil {
						panic(err)
					}

					if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
						ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
						ID:         optionHeader.ID,
						Type:       protocol.NEGOTIATION_TYPE_REPLY_INFO,
						Length:     uint32(replyInfo.Len()),
					}); err != nil {
						panic(err)
					}

					if _, err := io.Copy(conn, replyInfo); err != nil {
						panic(err)
					}
				}
			}
		}()
	}
}
