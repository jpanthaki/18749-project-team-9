// tests for client
package client_test

import (
	"18749-team9/client"
	"18749-team9/types"
	"encoding/json"
	"net"
	"testing"
	"time"
)

// Mock server, replies with message
func startMockServer(t *testing.T, rid string) (addr string, stop func()) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		dec := json.NewDecoder(conn)
		enc := json.NewEncoder(conn)

		var msg types.Message
		if err := dec.Decode(&msg); err != nil {
			return
		}

		// send message back as response
		resp := types.Response{
			Type:     "client",
			Id:       rid,
			ReqNum:   msg.ReqNum,
			Response: msg.Message,
		}
		_ = enc.Encode(resp)
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

// launch a client and connect to servers with unreachable addresses
func TestClientTimeout(t *testing.T) {
	opts := client.Options{
		S1Addr:      "127.0.0.1:59999", // random address
		S2Addr:      "127.0.0.1:60000",
		S3Addr:      "127.0.0.1:60001",
		ID:          "C1",
		StartingReq: 1,
		Timeout:     3 * time.Second,
	}

	cl, err := client.New(opts)
	if err != nil {
		t.Fatalf("client.New failed: %v", err)
	}
	defer cl.Close()

	_, err = cl.SendAll("Init")
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
}

// send message to three servers, ignore all but the first reply
func TestClientSendAll(t *testing.T) {
	addr1, stop1 := startMockServer(t, "S1")
	defer stop1()
	addr2, stop2 := startMockServer(t, "S2")
	defer stop2()
	addr3, stop3 := startMockServer(t, "S3")
	defer stop3()

	opts := client.Options{
		S1Addr:      addr1,
		S2Addr:      addr2,
		S3Addr:      addr3,
		ID:          "C1",
		StartingReq: 1,
		Timeout:     5 * time.Second,
	}

	c, err := client.New(opts)
	if err != nil {
		t.Fatalf("client init error: %v", err)
	}
	defer c.Close()

	// send message to all servers, returns after first response
	msg := "test"
	resp, err := c.SendAll(msg)
	if err != nil {
		t.Fatalf("SendAll failed: %v", err)
	}

	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.Response != msg {
		t.Errorf("unexpected response: %v", resp.Response)
	}
}
