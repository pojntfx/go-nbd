package main

import (
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
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

	c, err := net.Dial(*network, *raddr)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	log.Println("Connected to", c.RemoteAddr())

	f, err := os.Open(*file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	var cfd uintptr
	switch c := c.(type) {
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

	// TODO: Implement client side of the the handshake in userspace and process values from server with `ioctl`s (for `GO` command only)

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
