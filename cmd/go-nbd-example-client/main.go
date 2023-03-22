package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/pojntfx/tapisk/pkg/client"
)

func main() {
	file := flag.String("file", "/dev/nbd0", "Path to device file to create")
	raddr := flag.String("raddr", "127.0.0.1:10809", "Remote address")
	network := flag.String("network", "tcp", "Remote network (e.g. `tcp` or `unix`)")
	name := flag.String("name", "default", "Export name")
	list := flag.Bool("list", false, "List the exports and exit")
	blockSize := flag.Uint("block-size", 0, "Block size to use; 0 uses the server's preferred block size")

	flag.Parse()

	conn, err := net.Dial(*network, *raddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	log.Println("Connected to", conn.RemoteAddr())

	if *list {
		exports, err := client.List(conn)
		if err != nil {
			panic(err)
		}

		if err := json.NewEncoder(os.Stdout).Encode(exports); err != nil {
			panic(err)
		}

		return
	}

	f, err := os.Open(*file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	go func() {
		for range sigCh {
			if err := client.Disconnect(f); err != nil {
				panic(err)
			}

			os.Exit(0)
		}
	}()

	if err := client.Connect(conn, f, &client.Options{
		ExportName: *name,
		BlockSize:  uint32(*blockSize),
	}); err != nil {
		panic(err)
	}
}
