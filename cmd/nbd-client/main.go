package main

import (
	"flag"
	"os"
	"syscall"
	"unsafe"
)

func main() {
	file := flag.String("file", "/dev/nbd0", "Path to device file to create")
	// raddr := flag.String("raddr", "localhost:10809", "Remote address")

	flag.Parse()

	f, err := os.OpenFile(*file, os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	addr := syscall.SockaddrInet4{}

	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), 0, uintptr(unsafe.Pointer(&addr)))
}
