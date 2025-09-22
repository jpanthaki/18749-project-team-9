package main

import (
	"18749-team9/types"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"flag"
)

// this is just to test client-server interaction for now...
func main() {
	id := flag.String("id", "client1", "id of the client")
	flag.Parse()
	serverAddr := "172.26.111.238:8080"
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		fmt.Println("Failed to connect to server:", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Connected to server at", serverAddr)
	fmt.Println("Enter message type (Init, CountUp, CountDown, Close) or 'exit' to quit:")

	reqNum := 0

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = input[:len(input)-1] // Remove newline
		if input == "exit" {
			fmt.Println("Exiting client.")
			break
		}

		msg := types.Message{
			Type:    "client",
			Id:      *id,
			ReqNum:  reqNum,
			Message: input,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			fmt.Println("Error marshaling message:", err)
			continue
		}
		_, err = conn.Write(data)
		if err != nil {
			fmt.Println("Error sending message:", err)
			break
		}

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Error reading response:", err)
			break
		}
		var resp types.Response
		err = json.Unmarshal(buf[:n], &resp)
		if err != nil {
			fmt.Println("Error unmarshaling response:", err)
			continue
		}
		fmt.Printf("Server response: %+v\n", resp)

		reqNum++
		if input == "Close" {
			fmt.Println("Connection closed by client request.")
			break
		}
	}
}
