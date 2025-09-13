package main

import (
	"18749-team9/lfd"
	"flag"
	"fmt"
	"log"
)

func main() {
	freq := flag.Int("freq", 2, "heartbeat frequency in seconds")
	id := flag.String("id", "LFD1", "LFD id")
	port := flag.Int("port", 9000, "server port")
	protocol := flag.String("protocol", "tcp", "protocol")
	flag.Parse()

	lfd_ex, err := lfd.NewLfd(*freq, *id, *port, *protocol)
	if err != nil {
		log.Fatal(err)
	}
	lfd_ex.Start()
	for {
		if lfd_ex.Status() == "running" {
			fmt.Println("LFD status:", lfd_ex.Status())
		}
	}
}
