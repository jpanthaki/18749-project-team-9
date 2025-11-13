package main

import (
	"18749-team9/helpers"
	server "18749-team9/server-passive"
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	id := flag.String("id", "S1", "id of the server")
	port := flag.Int("port", 8080, "port of the server")
	lfdPort := flag.Int("lfdPort", 9000, "port of the local failure detector (0 if none)")
	protocol := flag.String("protocol", "tcp", "protocol of the server ")
	s1Addr := flag.String("s1Addr", "127.0.0.1", "address of the s1 server")
	s2Addr := flag.String("s2Addr", "127.0.0.1", "address of the s2 server")
	s3Addr := flag.String("s3Addr", "127.0.0.1", "address of the s3 server")
	flag.Parse()

	var isLeader bool
	if *id == "S1" {
		isLeader = true
	} else {
		isLeader = false
	}

	peerMap := map[string]string{
		"S1": *s1Addr,
		"S2": *s2Addr,
		"S3": *s3Addr,
	}

	fmt.Printf("Starting server at address %s:%d with ID: %s, Protocol: %s\n", helpers.GetLocalIP(), *port, *id, *protocol)

	sv, err := server.NewServer(*id, *port, *protocol, *lfdPort, isLeader, 4000, peerMap)
	if err != nil {
		panic(err)
	}
	err = sv.Start()
	if err != nil {
		panic(err)
	}

	fmt.Println("Server is ready. Type 'stop' to stop the server.")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		cmd := strings.TrimSpace(scanner.Text())
		if cmd == "stop" {
			err := sv.Stop()
			if err != nil {
				fmt.Println("Error stopping server:", err)
			} else {
				fmt.Println("Server stopped.")
			}
			break
		} else {
			fmt.Println("Unknown command. Type 'stop' to stop the server.")
		}
	}
}
