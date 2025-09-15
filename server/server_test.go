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
	srv, err := NewServer("test", 0, "tcp")
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
	srv, err := NewServer("test", 0, "tcp")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		_ = srv.Start()
	}()

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
	clients := []clientCase{
		{"client1", "Init", "Initialized"},
		{"client2", "CountUp", "State: 1"},
		{"client3", "CountDown", "State: 0"},
	}

	doneCh := make(chan error, len(clients))

	for i, cc := range clients {
		go func(idx int, cc clientCase) {
			conn, err := net.Dial("tcp", ":"+strconv.Itoa(port))
			if err != nil {
				doneCh <- err
				return
			}
			defer conn.Close()

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

			if resp.Id != cc.id {
				doneCh <- fmt.Errorf("client %s: expected id %s, got %s", cc.id, cc.id, resp.Id)
				return
			}
			if cc.msg == "Init" && resp.Response != "Initialized" {
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
			doneCh <- nil
		}(i, cc)
	}

	for range clients {
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
	srv, err := NewServer("test", 0, "tcp")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		_ = srv.Start()
	}()

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

	conn, err := net.Dial("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		t.Fatalf("Failed to connect LFD to server: %v", err)
	}
	defer conn.Close()

	id := "lfd1"
	heartbeatCount := 3

	for i := 0; i < heartbeatCount; i++ {
		msg := types.Message{
			Type:    "lfd",
			Id:      id,
			ReqNum:  i,
			Message: "heartbeat",
		}
		data, _ := json.Marshal(msg)
		_, err = conn.Write(data)
		if err != nil {
			t.Fatalf("Failed to send heartbeat %d: %v", i, err)
		}

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("Failed to read heartbeat response %d: %v", i, err)
		}
		var resp types.Response
		err = json.Unmarshal(buf[:n], &resp)
		if err != nil {
			t.Fatalf("Failed to unmarshal heartbeat response %d: %v", i, err)
		}

		if resp.Id != id {
			t.Errorf("Heartbeat %d: expected id %s, got %s", i, id, resp.Id)
		}
		if resp.ReqNum != i {
			t.Errorf("Heartbeat %d: expected ReqNum %d, got %d", i, i, resp.ReqNum)
		}
		if resp.Response != fmt.Sprintf("%d", i) {
			t.Errorf("Heartbeat %d: expected response '%d', got '%s'", i, i, resp.Response)
		}
		time.Sleep(100 * time.Millisecond)
	}

	err = srv.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}
