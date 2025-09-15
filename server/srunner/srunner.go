package main

import (
	"18749-team9/server"
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	id := flag.String("id", "S1", "id of the server")
	port := flag.Int("port", 8080, "port of the server")
	lfdPort := flag.Int("lfdPort", 0, "port of the local failure detector (0 if none)")
	protocol := flag.String("protocol", "tcp", "protocol of the server (tcp/udp)")
	flag.Parse()

	fmt.Printf("Starting server with ID: %s, Port: %d, Protocol: %s\n", *id, *port, *protocol)

	sv, err := server.NewServer("S1", *port, *protocol, *lfdPort)
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
