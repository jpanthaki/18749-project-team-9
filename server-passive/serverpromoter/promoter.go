package main

import (
	"18749-team9/types"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
)

func main() {
	serverAddrs := map[string]string{
		"S1": "127.0.0.1:8081",
		"S2": "127.0.0.1:8082",
		"S3": "127.0.0.1:8083",
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter (S1, S2, S3) to send promotion message...")

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = input[:len(input)-1]

		conn, _ := net.Dial("tcp", serverAddrs[input])

		msg := types.Message{
			Type:    "rm",
			Id:      input,
			ReqNum:  0,
			Message: "Promote",
		}
		data, _ := json.Marshal(msg)

		conn.Write(data)
	}
}
