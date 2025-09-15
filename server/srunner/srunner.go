package main

import (
	"18749-team9/server"
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	sv, err := server.NewServer("S1", 8080, "tcp")
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
