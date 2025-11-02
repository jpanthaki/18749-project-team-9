package server

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServerLifecycle(t *testing.T) {
	peers := map[string]string{
		"S2": "123.123.123.123:1234",
		"S3": "123.123.123.123:1234",
	}
	srv, err := NewServer("S1", 0, "tcp", 0, "active", false, 10000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server in a goroutine since Start blocks
	go func() {
		err := srv.Start()
		if err != nil {
			// Ignore error if stopping
		}
	}()

	time.Sleep(100 * time.Millisecond)

	status := srv.Status()
	if status != "running" {
		t.Errorf("Expected status 'running', got '%s'", status)
	}

	err = srv.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	status = srv.Status()
	if status != "stopped" {
		t.Errorf("Expected status 'stopped', got '%s'", status)
	}
}

func TestServerWithThreeClients(t *testing.T) {
	peers := map[string]string{
		"S2": "123.123.123.123:1234",
		"S3": "123.123.123.123:1234",
	}
	srv, err := NewServer("S1", 0, "tcp", 0, "active", false, 10000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	//go func() {
	//	_ = srv.Start()
	//}()
	_ = srv.Start()

	// Get the actual port assigned
	serverImpl := srv.(*server)
	for serverImpl.listener == nil {
		time.Sleep(10 * time.Millisecond)
	}
	port := serverImpl.listener.Addr().(*net.TCPAddr).Port
	fmt.Println(port)

	// Wait for the server to be ready
	for !srv.Ready() {
		time.Sleep(10 * time.Millisecond)
	}

	type clientCase struct {
		id     string
		msg    string
		expect string
	}
	clients := map[string][]clientCase{
		"client1": {
			{"client1", "Init", "Client: client1 Initialized, State: 0"},
			{"client1", "CountUp", "Client: client1 Counted Up, State: 1"},
			{"client1", "CountDown", "Client: client1 Counted Down, State: 0"},
			{"client1", "Close", "Client: client1 Closed"},
		},
		"client2": {
			{"client2", "Init", "Client: client2 Initialized, State: 0"},
			{"client2", "CountUp", "Client: client2 Counted Up, State: 1"},
			{"client2", "CountDown", "Client: client2 Counted Down, State: 0"},
			{"client2", "Close", "Client: client2 Closed"},
		},
		"client3": {
			{"client3", "Init", "Client: client3 Initialized, State: 0"},
			{"client3", "CountUp", "Client: client3 Counted Up, State: 1"},
			{"client3", "CountDown", "Client: client3 Counted Down, State: 0"},
			{"client3", "Close", "Client: client3 Closed"},
		},
	}

	doneCh := make(chan error, len(clients))

	for clientID, msgs := range clients {
		go func(clientID string, msgs []clientCase) {
			conn, err := net.Dial("tcp", ":"+strconv.Itoa(port))
			if err != nil {
				doneCh <- err
				return
			}
			defer func() {
				if err := conn.Close(); err != nil {
					t.Errorf("Error closing client connection: %v", err)
				}
			}()

			for idx, cc := range msgs {
				msg := types.Message{
					Type:    "client",
					Id:      cc.id,
					ReqNum:  idx,
					Message: cc.msg,
				}

				data, _ := json.Marshal(msg)
				_, err = conn.Write(data)
				if err != nil {
					doneCh <- err
					return
				}
				buf := make([]byte, 1024)
				n, err := conn.Read(buf)
				if err != nil {
					doneCh <- err
					return
				}
				var resp types.Response
				err = json.Unmarshal(buf[:n], &resp)
				if err != nil {
					doneCh <- err
					return
				}

				if resp.Id != "S1" {
					doneCh <- fmt.Errorf("Client %s: expected id %s, got %s", clientID, cc.id, resp.Id)
					return
				}
				if resp.ReqNum != idx {
					doneCh <- fmt.Errorf("Client %s: expected ReqNum %d, got %d", clientID, idx, resp.ReqNum)
					return
				}
				if cc.msg == "Init" && resp.Response != fmt.Sprintf("Client: %s Initialized, State: 0", cc.id) {
					doneCh <- fmt.Errorf("client %s: expected response 'Initialized', got '%s'", cc.id, resp.Response)
					return
				}
				if cc.msg == "CountUp" && !strings.Contains(resp.Response, "State: 1") {
					doneCh <- fmt.Errorf("client %s: expected response to contain 'State: 1', got '%s'", cc.id, resp.Response)
					return
				}
				if cc.msg == "CountDown" && !strings.Contains(resp.Response, "State: 0") {
					doneCh <- fmt.Errorf("client %s: expected response to contain 'State: 0', got '%s'", cc.id, resp.Response)
					return
				}
			}
			doneCh <- nil
		}(clientID, msgs)
	}

	for i := 0; i < len(clients); i++ {
		err := <-doneCh
		if err != nil {
			t.Errorf("Client error: %v", err)
		}
	}

	err = srv.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}

func TestServerWithLFD(t *testing.T) {
	ln, err := net.Listen("tcp", ":9090")
	if err != nil {
		fmt.Println("LFD: Failed to start mock LFD:", err)
		return
	}
	defer ln.Close()
	fmt.Println("LFD: Mock LFD listening on port 9090")

	peers := map[string]string{
		"S2": "123.123.123.123:1234",
		"S3": "123.123.123.123:1234",
	}
	srv, err := NewServer("S1", 0, "tcp", 9090, "active", false, 10000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		err := srv.Start()
		if err != nil {
			// Ignore error if stopping
		}
	}()

	c, err := ln.Accept()
	if err != nil {
		fmt.Println("LFD: Failed to accept connection:", err)
		return
	}
	defer c.Close()
	id := "lfd1"
	for i := 0; i < 5; i++ {
		msg := types.Message{
			Type:    "lfd",
			Id:      id,
			ReqNum:  i,
			Message: "heartbeat",
		}
		data, _ := json.Marshal(msg)
		_, err = c.Write(data)
		if err != nil {
			t.Fatalf("Failed to send heartbeat %d: %v", i, err)
		}
		buf := make([]byte, 1024)
		n, err := c.Read(buf)
		if err != nil {
			t.Fatalf("Failed to read heartbeat response %d: %v", i, err)
		}
		var resp types.Response
		err = json.Unmarshal(buf[:n], &resp)
		if err != nil {
			t.Fatalf("Failed to unmarshal heartbeat response %d: %v", i, err)
		}

		if resp.Id != "S1" {
			t.Errorf("Heartbeat %d: expected id %s, got %s", i, id, resp.Id)
		}
		if resp.ReqNum != i {
			t.Errorf("Heartbeat %d: expected ReqNum %d, got %d", i, i, resp.ReqNum)
		}
		if resp.Response != fmt.Sprintf("%d", i) {
			t.Errorf("Heartbeat %d: expected response '%d', got '%s'", i, i, resp.Response)
		}
		time.Sleep(2000 * time.Millisecond)
	}

	err = srv.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}
