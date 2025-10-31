package main

import (
	"18749-team9/helpers"
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
	lfdPort := flag.Int("lfdPort", 9000, "port of the local failure detector (0 if none)")
	protocol := flag.String("protocol", "tcp", "protocol of the server ")
	flag.Parse()

	fmt.Printf("Starting server at address %s:%d with ID: %s, Protocol: %s\n", helpers.GetLocalIP(), *port, *id, *protocol)

	sv, err := server.NewServer(*id, *port, *protocol, *lfdPort)
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
