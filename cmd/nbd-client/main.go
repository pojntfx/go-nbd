package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/pojntfx/tapisk/pkg/protocol"
	"github.com/pojntfx/tapisk/pkg/server"
)

var (
	errUnsupportedNetwork = errors.New("unsupported network")
	errUnknownReply       = errors.New("unknown reply")
	errUnknownInfo        = errors.New("unknown info")
	errUnknownErr         = errors.New("unknown error")
)

const (
	// See /usr/include/linux/nbd.h
	NEGOTIATION_IOCTL_SET_SOCK        = 43776
	NEGOTIATION_IOCTL_SET_SIZE_BLOCKS = 43783
	NEGOTIATION_IOCTL_DO_IT           = 43779

	TRANSMISSION_IOCTL_DISCONNECT = 43784
)

func main() {
	file := flag.String("file", "/dev/nbd0", "Path to device file to create")
	raddr := flag.String("raddr", "127.0.0.1:10809", "Remote address")
	network := flag.String("network", "tcp", "Remote network (e.g. `tcp` or `unix`)")
	export := flag.String("export", "default", "Export name to request")
	list := flag.Bool("list", false, "List the exports and exit")

	flag.Parse()

	conn, err := net.Dial(*network, *raddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	log.Println("Connected to", conn.RemoteAddr())

	f, err := os.Open(*file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var cfd uintptr
	switch c := conn.(type) {
	case *net.TCPConn:
		file, err := c.File()
		if err != nil {
			panic(err)
		}

		cfd = uintptr(file.Fd())
	case *net.UnixConn:
		file, err := c.File()
		if err != nil {
			panic(err)
		}

		cfd = uintptr(file.Fd())
	default:
		panic(errUnsupportedNetwork)
	}

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		NEGOTIATION_IOCTL_SET_SOCK,
		uintptr(cfd),
	); err != 0 {
		panic(err)
	}

	var newstyleHeader protocol.NegotiationNewstyleHeader
	if err := binary.Read(conn, binary.BigEndian, &newstyleHeader); err != nil {
		panic(err)
	}

	if newstyleHeader.OldstyleMagic != protocol.NEGOTIATION_MAGIC_OLDSTYLE {
		panic(server.ErrInvalidMagic)
	}

	if newstyleHeader.OptionMagic != protocol.NEGOTIATION_MAGIC_OPTION {
		panic(server.ErrInvalidMagic)
	}

	if _, err := conn.Write(make([]byte, 4)); err != nil { // Send client flags (uint32)
		panic(err)
	}

	if *list {
		if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
			OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
			ID:          protocol.NEGOTIATION_ID_OPTION_LIST,
			Length:      0,
		}); err != nil {
			panic(err)
		}

		var replyHeader protocol.NegotiationReplyHeader
		if err := binary.Read(conn, binary.BigEndian, &replyHeader); err != nil {
			panic(err)
		}

		if replyHeader.ReplyMagic != protocol.NEGOTIATION_MAGIC_REPLY {
			panic(server.ErrInvalidMagic)
		}

		infoRaw := make([]byte, replyHeader.Length)
		if _, err := io.ReadFull(conn, infoRaw); err != nil {
			panic(err)
		}

		info := bytes.NewBuffer(infoRaw)

		exportNames := []string{}
		for {
			var exportNameLength uint32
			if err := binary.Read(info, binary.BigEndian, &exportNameLength); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				panic(err)
			}

			exportName := make([]byte, exportNameLength)
			if _, err := io.ReadFull(info, exportName); err != nil {
				panic(err)
			}

			exportNames = append(exportNames, string(exportName))
		}

		if err := json.NewEncoder(os.Stdout).Encode(exportNames); err != nil {
			panic(err)
		}

		if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
			OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
			ID:          protocol.NEGOTIATION_ID_OPTION_ABORT,
			Length:      0,
		}); err != nil {
			panic(err)
		}

		return
	}

	if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
		OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
		ID:          protocol.NEGOTIATION_ID_OPTION_GO,
		Length:      0,
	}); err != nil {
		panic(err)
	}

	exportName := []byte(*export)

	if err := binary.Write(conn, binary.BigEndian, uint32(len(exportName))); err != nil {
		panic(err)
	}

	if _, err := conn.Write([]byte(exportName)); err != nil {
		panic(err)
	}

	if err := binary.Write(conn, binary.BigEndian, uint16(0)); err != nil { // Send information request count (uint16)
		panic(err)
	}

	size := uint64(0)
	preferredBlockSize := uint32(1)

n:
	for {
		var replyHeader protocol.NegotiationReplyHeader
		if err := binary.Read(conn, binary.BigEndian, &replyHeader); err != nil {
			panic(err)
		}

		if replyHeader.ReplyMagic != protocol.NEGOTIATION_MAGIC_REPLY {
			panic(server.ErrInvalidMagic)
		}

		switch replyHeader.Type {
		case protocol.NEGOTIATION_TYPE_REPLY_INFO:
			infoRaw := make([]byte, replyHeader.Length)
			if _, err := io.ReadFull(conn, infoRaw); err != nil {
				panic(err)
			}

			var infoType uint16
			if err := binary.Read(bytes.NewBuffer(infoRaw), binary.BigEndian, &infoType); err != nil {
				panic(err)
			}

			switch infoType {
			case protocol.NEGOTIATION_TYPE_INFO_EXPORT:
				var info protocol.NegotiationReplyInfo
				if err := binary.Read(bytes.NewBuffer(infoRaw), binary.BigEndian, &info); err != nil {
					panic(err)
				}

				size = info.Size
			case protocol.NEGOTIATION_TYPE_INFO_NAME:
				// Discard export name
			case protocol.NEGOTIATION_TYPE_INFO_DESCRIPTION:
				// Discard export description
			case protocol.NEGOTIATION_TYPE_INFO_BLOCKSIZE:
				var info protocol.NegotiationReplyBlockSize
				if err := binary.Read(bytes.NewBuffer(infoRaw), binary.BigEndian, &info); err != nil {
					panic(err)
				}

				preferredBlockSize = info.PreferredBlockSize
			default:
				panic(errUnknownInfo)
			}
		case protocol.NEGOTIATION_TYPE_REPLY_ACK:
			break n
		case protocol.NEGOTIATION_TYPE_REPLY_ERR_UNKNOWN:
			panic(errUnknownErr)
		default:
			panic(errUnknownReply)
		}
	}

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		NEGOTIATION_IOCTL_SET_SIZE_BLOCKS,
		uintptr(size/uint64(preferredBlockSize)),
	); err != 0 {
		panic(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		for range sigCh {
			if _, _, err := syscall.Syscall(
				syscall.SYS_IOCTL,
				f.Fd(),
				TRANSMISSION_IOCTL_DISCONNECT,
				0,
			); err != 0 {
				panic(err)
			}

			os.Exit(0)
		}
	}()

	if _, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		f.Fd(),
		NEGOTIATION_IOCTL_DO_IT,
		0,
	); err != 0 {
		panic(err)
	}
}
