package main

import (
	"18749-team9/client"
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	// Simple manual flag parsing to keep deps minimal
	addr := "127.0.0.1:8080"
	id := "C1"
	serverID := "S1"
	startReq := 101
	timeout := 3 * time.Second

	// Allow override via env or quick edits if you want,
	// or switch to the 'flag' package like earlier suggestion.

	cl, err := client.New(client.Options{
		Addr:        addr,
		ID:          id,
		ServerID:    serverID,
		StartingReq: startReq,
		Timeout:     timeout,
	})
	if err != nil {
		fmt.Println("Client init error:", err)
		os.Exit(1)
	}
	defer cl.Close()

	fmt.Printf("Connected %s → %s (%s)\n", id, addr, serverID)
	fmt.Println("Enter: Init | CountUp | CountDown | Close | exit")

	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !in.Scan() {
			if err := in.Err(); err != nil {
				fmt.Println("Stdin error:", err)
			}
			return
		}
		cmd := strings.TrimSpace(in.Text())
		if cmd == "" {
			continue
		}
		if strings.EqualFold(cmd, "exit") {
			fmt.Println("Exiting client.")
			return
		}

		resp, err := cl.Send(cmd)
		if err != nil {
			fmt.Println("Send error:", err)
			return
		}
		fmt.Printf("Server response: %+v\n", *resp)

		if strings.EqualFold(cmd, "Close") {
			fmt.Println("Connection closed by client request.")
			return
		}
	}
}
