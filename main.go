package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/pojntfx/tapisk/pkg/protocol"
)

var (
	errFixedNewstyleNotSet = errors.New("fixed newstyle client flag not set")
	errNoZeroesNotSet      = errors.New("no zeroes client flag not set")

	errOptionUnsupported  = errors.New("option is unsupported")
	errCommandUnsupported = errors.New("command is unsupported")

	errInvalidOptionMagic  = errors.New("invalid option magic")
	errInvalidRequestMagic = errors.New("invalid request magic")
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
				panic(errFixedNewstyleNotSet)
			}

			if clientFlags.ClientFlags&protocol.NbdClientFlagNoZeroes == 1 {
				panic(errNoZeroesNotSet)
			}

			// Option haggling
		l:
			for {
				var option protocol.NbdOption
				if err := binary.Read(conn, binary.BigEndian, &option); err != nil {
					panic(err)
				}

				if option.Magic != protocol.NbdOptionMagic {
					panic(errInvalidOptionMagic)
				}

				if _, err := io.CopyN(io.Discard, conn, int64(option.Length)); err != nil {
					panic(err)
				}

				switch option.Option {
				case protocol.NbdOptionGo:
					if err := binary.Write(conn, binary.BigEndian, protocol.NbdOptionReply{
						Magic:  protocol.NbdOptionReplyMagic,
						Option: option.Option,
						Typ:    protocol.NbdReplyAck,
					}); err != nil {
						panic(err)
					}

					// FIXME: Send NbdExportInfo before continuing to transmission

					break l
				case protocol.NbdOptionAbort:
					if err := binary.Write(conn, binary.BigEndian, protocol.NbdOptionReply{
						Magic:  protocol.NbdOptionReplyMagic,
						Option: option.Option,
						Typ:    protocol.NbdReplyAck,
					}); err != nil {
						panic(err)
					}

					return
				default:
					// FIXME: This isn't compliant, we should be sending back a `NbdReplyError` here instead of just closing the connection and also handle different export names

					panic(errOptionUnsupported)
				}
			}

			// Transmission
			for {
				var request protocol.NbdRequest
				if err := binary.Read(conn, binary.BigEndian, &request); err != nil {
					panic(err)
				}

				if request.Magic != protocol.NbdRequestMagic {
					panic(errInvalidRequestMagic)
				}

				var b *bytes.Buffer
				if request.Length > 0 {
					b = bytes.NewBuffer(make([]byte, request.Length))
					if _, err := io.CopyN(b, conn, int64(request.Length)); err != nil {
						panic(err)
					}
				}

				switch request.Command {
				case protocol.NbdCmdDisconnect:
					return
				default:
					panic(errCommandUnsupported)
				}
			}
		}()
	}
}
