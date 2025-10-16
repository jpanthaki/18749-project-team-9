package main

import (
	"18749-team9/helpers"
	"18749-team9/lfd"
	"flag"
	"fmt"
	"log"
)

func main() {
	freq := flag.Int("freq", 2, "heartbeat frequency in seconds")
	id := flag.String("id", "LFD1", "LFD id")
	serverID := flag.String("serverid", "S1", "Server replica id")
	port := flag.Int("port", 9000, "LFD port")
	gfdPort := flag.Int("gfdport", 8000, "GFD port")
	protocol := flag.String("protocol", "tcp", "protocol")
	flag.Parse()

	fmt.Println("LFD IP: ", helpers.GetLocalIP())

	lfd_ex, err := lfd.NewLfd(*freq, *id, *serverID, *port, *protocol, *gfdPort)
	if err != nil {
		log.Fatal(err)
	}

	if err := lfd_ex.Start(); err != nil {
		log.Fatal(err)
	}
}
