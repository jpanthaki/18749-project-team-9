package registry

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	reg, _ := NewRegistry(18749, "team9")

	reg.Start()

	time.Sleep(1 * time.Second)

	message := struct {
		Header string            `json:"header"`
		Body   map[string]string `json:"body"`
	}{
		Header: "discover",
		Body: map[string]string{
			"token": "team9",
		},
	}

	addr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:18749")
	fmt.Println(addr)
	if err != nil {
		fmt.Printf("Error resolving UDP address: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		fmt.Printf("Error dialing UDP: %v\n", err)
		return
	}
	defer conn.Close()

	bytes, err := json.Marshal(message)
	if err != nil {
		fmt.Printf("Error marshalling message: %v\n", err)
		return
	}

	_, err = conn.Write(bytes)
	if err != nil {
		fmt.Printf("Error sending broadcast message: %v\n", err)
	} else {
		fmt.Println("Broadcasted:", message)
	}

	time.Sleep(5 * time.Second)

	reg.Stop()

}
