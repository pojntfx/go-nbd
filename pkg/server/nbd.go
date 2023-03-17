package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"

	"github.com/pojntfx/tapisk/pkg/backend"
	"github.com/pojntfx/tapisk/pkg/protocol"
)

var (
	ErrInvalidMagic = errors.New("invalid magic")
)

func Serve(listener net.Listener, backend backend.Backend) error {
	size, err := backend.Size()
	if err != nil {
		panic(err)
	}

	clients := 0
	for {
		conn, err := listener.Accept()
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

		n:
			for {
				var optionHeader protocol.NegotiationOptionHeader
				if err := binary.Read(conn, binary.BigEndian, &optionHeader); err != nil {
					panic(err)
				}

				if optionHeader.OptionMagic != protocol.NEGOTIATION_MAGIC_OPTION {
					panic(ErrInvalidMagic)
				}

				switch optionHeader.ID {
				case protocol.NEGOTIATION_ID_OPTION_INFO, protocol.NEGOTIATION_ID_OPTION_GO:
					var exportNameLength uint32
					if err := binary.Read(conn, binary.BigEndian, &exportNameLength); err != nil {
						panic(err)
					}

					exportName := make([]byte, exportNameLength)
					if _, err := io.ReadFull(conn, exportName); err != nil {
						panic(err)
					}

					{
						var informationRequestCount uint16
						if err := binary.Read(conn, binary.BigEndian, &informationRequestCount); err != nil {
							panic(err)
						}

						_, err := io.CopyN(io.Discard, conn, 2*int64(informationRequestCount)) // Discard information requests (uint16s)
						if err != nil {
							panic(err)
						}
					}

					{
						info := &bytes.Buffer{}
						if err := binary.Write(info, binary.BigEndian, protocol.NegotiationReplyInfo{
							Type:              protocol.NEGOTIATION_TYPE_INFO_EXPORT,
							Size:              uint64(size),
							TransmissionFlags: 0,
						}); err != nil {
							panic(err)
						}

						if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
							ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
							ID:         optionHeader.ID,
							Type:       protocol.NEGOTIATION_TYPE_REPLY_INFO,
							Length:     uint32(info.Len()),
						}); err != nil {
							panic(err)
						}

						if _, err := io.Copy(conn, info); err != nil {
							panic(err)
						}
					}

					{
						info := &bytes.Buffer{}
						if err := binary.Write(info, binary.BigEndian, protocol.NegotiationReplyNameHeader{
							Type: protocol.NEGOTIATION_TYPE_INFO_NAME,
						}); err != nil {
							panic(err)
						}

						if err := binary.Write(info, binary.BigEndian, exportName); err != nil {
							panic(err)
						}

						if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
							ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
							ID:         optionHeader.ID,
							Type:       protocol.NEGOTIATION_TYPE_REPLY_INFO,
							Length:     uint32(info.Len()),
						}); err != nil {
							panic(err)
						}

						if _, err := io.Copy(conn, info); err != nil {
							panic(err)
						}
					}

					{
						info := &bytes.Buffer{}
						if err := binary.Write(info, binary.BigEndian, protocol.NegotiationReplyDescriptionHeader{
							Type: protocol.NEGOTIATION_TYPE_INFO_DESCRIPTION,
						}); err != nil {
							panic(err)
						}

						if err := binary.Write(info, binary.BigEndian, []byte("Tapisk export")); err != nil {
							panic(err)
						}

						if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
							ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
							ID:         optionHeader.ID,
							Type:       protocol.NEGOTIATION_TYPE_REPLY_INFO,
							Length:     uint32(info.Len()),
						}); err != nil {
							panic(err)
						}

						if _, err := io.Copy(conn, info); err != nil {
							panic(err)
						}
					}

					{
						info := &bytes.Buffer{}
						if err := binary.Write(info, binary.BigEndian, protocol.NegotiationReplyBlockSize{
							Type:               protocol.NEGOTIATION_TYPE_INFO_BLOCKSIZE,
							MinimumBlockSize:   1,
							PreferredBlockSize: 32 * 1024,
							MaximumBlockSize:   128 * 1024 * 1024,
						}); err != nil {
							panic(err)
						}

						if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
							ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
							ID:         optionHeader.ID,
							Type:       protocol.NEGOTIATION_TYPE_REPLY_INFO,
							Length:     uint32(info.Len()),
						}); err != nil {
							panic(err)
						}

						if _, err := io.Copy(conn, info); err != nil {
							panic(err)
						}
					}

					if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
						ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
						ID:         optionHeader.ID,
						Type:       protocol.NEGOTIATION_TYPE_REPLY_ACK,
						Length:     0,
					}); err != nil {
						panic(err)
					}

					if optionHeader.ID == protocol.NEGOTIATION_ID_OPTION_GO {
						break n
					}
				case protocol.NEGOTIATION_ID_OPTION_ABORT:
					if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
						ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
						ID:         optionHeader.ID,
						Type:       protocol.NEGOTIATION_TYPE_REPLY_ACK,
						Length:     0,
					}); err != nil {
						panic(err)
					}

					return
				case protocol.NEGOTIATION_ID_OPTION_LIST:
					{
						info := &bytes.Buffer{}

						exportName := []byte("default")

						if err := binary.Write(info, binary.BigEndian, exportName); err != nil {
							panic(err)
						}

						if err := binary.Write(info, binary.BigEndian, uint32(len(exportName))); err != nil {
							panic(err)
						}

						if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
							ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
							ID:         optionHeader.ID,
							Type:       protocol.NEGOTIATION_TYPE_REPLY_SERVER,
							Length:     uint32(info.Len()),
						}); err != nil {
							panic(err)
						}

						if _, err := io.Copy(conn, info); err != nil {
							panic(err)
						}
					}

					if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
						ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
						ID:         optionHeader.ID,
						Type:       protocol.NEGOTIATION_TYPE_REPLY_ACK,
						Length:     0,
					}); err != nil {
						panic(err)
					}

					return
				default:
					_, err := io.CopyN(io.Discard, conn, int64(optionHeader.Length)) // Discard the unknown option
					if err != nil {
						panic(err)
					}

					if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationReplyHeader{
						ReplyMagic: protocol.NEGOTIATION_MAGIC_REPLY,
						ID:         optionHeader.ID,
						Type:       protocol.NEGOTIATION_TYPE_REPLY_ERR_UNSUPPORTED,
						Length:     0,
					}); err != nil {
						panic(err)
					}
				}
			}

			// Transmission
			for {
				var requestHeader protocol.TransmissionRequestHeader
				if err := binary.Read(conn, binary.BigEndian, &requestHeader); err != nil {
					panic(err)
				}

				if requestHeader.RequestMagic != protocol.TRANSMISSION_MAGIC_REQUEST {
					panic(ErrInvalidMagic)
				}

				switch requestHeader.Type {
				case protocol.TRANSMISSION_TYPE_REQUEST_READ:
					if err := binary.Write(conn, binary.BigEndian, protocol.TransmissionReplyHeader{
						ReplyMagic: protocol.TRANSMISSION_MAGIC_REPLY,
						Error:      0,
						Handle:     requestHeader.Handle,
					}); err != nil {
						panic(err)
					}

					if _, err := io.CopyN(conn, io.NewSectionReader(backend, int64(requestHeader.Offset), int64(requestHeader.Length)), int64(requestHeader.Length)); err != nil {
						panic(err)
					}
				case protocol.TRANSMISSION_TYPE_REQUEST_WRITE:
					if _, err := io.CopyN(io.NewOffsetWriter(backend, int64(requestHeader.Offset)), conn, int64(requestHeader.Length)); err != nil {
						panic(err)
					}

					if err := binary.Write(conn, binary.BigEndian, protocol.TransmissionReplyHeader{
						ReplyMagic: protocol.TRANSMISSION_MAGIC_REPLY,
						Error:      0,
						Handle:     requestHeader.Handle,
					}); err != nil {
						panic(err)
					}
				case protocol.TRANSMISSION_TYPE_REQUEST_DISC:
					if err := backend.Sync(); err != nil {
						panic(err)
					}

					return
				default:
					_, err := io.CopyN(io.Discard, conn, int64(requestHeader.Length)) // Discard the unknown command
					if err != nil {
						panic(err)
					}

					if err := binary.Write(conn, binary.BigEndian, protocol.TransmissionReplyHeader{
						ReplyMagic: protocol.TRANSMISSION_MAGIC_REPLY,
						Error:      protocol.TRANSMISSION_ERROR_EINVAL,
						Handle:     requestHeader.Handle,
					}); err != nil {
						panic(err)
					}
				}
			}
		}()
	}
}
