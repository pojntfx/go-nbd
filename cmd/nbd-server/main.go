package main

import (
	"flag"
	"log"
	"net"
	"os"

	"github.com/pojntfx/tapisk/pkg/backend"
	"github.com/pojntfx/tapisk/pkg/server"
)

func main() {
	file := flag.String("file", "tapisk.img", "Path to file to expose")
	laddr := flag.String("laddr", ":10809", "Listen address")
	network := flag.String("network", "tcp", "Listen network (e.g. `tcp` or `unix`)")
	ro := flag.Bool("ro", false, "Whether the export should be read-only")

	flag.Parse()

	l, err := net.Listen(*network, *laddr)
	if err != nil {
		panic(err)
	}
	defer l.Close()

	log.Println("Listening on", l.Addr())

	f, err := os.OpenFile(*file, os.O_RDWR, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	b := backend.NewFileBackend(f)

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

			if err := server.Handle(conn, b, &server.Options{
				ReadOnly: *ro,
			}); err != nil {
				panic(err)
			}
		}()
	}
}
