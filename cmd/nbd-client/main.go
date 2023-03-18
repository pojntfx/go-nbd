package main

import (
	"encoding/binary"
	"errors"
	"flag"
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
)

const (
	// See /usr/include/linux/nbd.h
	NEGOTIATION_IOCTL_SET_SOCK = 43776
	NEGOTIATION_IOCTL_DO_IT    = 43779

	TRANSMISSION_IOCTL_DISCONNECT = 43784
)

func main() {
	file := flag.String("file", "/dev/nbd0", "Path to device file to create")
	raddr := flag.String("raddr", "127.0.0.1:10809", "Remote address")
	network := flag.String("network", "tcp", "Remote network (e.g. `tcp` or `unix`)")

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

	if err := binary.Write(conn, binary.BigEndian, protocol.NegotiationOptionHeader{
		OptionMagic: protocol.NEGOTIATION_MAGIC_OPTION,
		ID:          protocol.NEGOTIATION_ID_OPTION_GO,
		Length:      0,
	}); err != nil {
		panic(err)
	}

	// TODO: Implement `NEGOTIATION_ID_OPTION_GO` in userspace

	var replyHeader protocol.NegotiationNewstyleHeader
	if err := binary.Read(conn, binary.BigEndian, &replyHeader); err != nil {
		panic(err)
	}

	if replyHeader.OptionMagic != protocol.NEGOTIATION_MAGIC_REPLY {
		panic(server.ErrInvalidMagic)
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
