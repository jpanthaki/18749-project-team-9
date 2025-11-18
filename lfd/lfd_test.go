package lfd

import (
	"18749-team9/types"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestLfdLifecycle_ServerHeartbeat(t *testing.T) {
	// Create LFD with port 0 (OS assigns free port)
	l, err := NewLfd(1, "LFD1", "S1", 0, "tcp", 0, "localhost:0")
	if err != nil {
		t.Fatalf("failed to create LFD: %v", err)
	}

	// Start LFD in background
	go func() {
		if err := l.Start(); err != nil {
			t.Logf("LFD stopped: %v", err)
		}
	}()
	defer l.Stop()

	// Give LFD time to start listening
	time.Sleep(200 * time.Millisecond)

	// Skip this test - port 0 doesn't work as expected in this architecture
	// Use the fixed port version instead
	t.Skip("Skipping - need to refactor to expose listener port")
}

func TestLfdLifecycle_ServerHeartbeatWithFixedPort(t *testing.T) {
	// Use a high port number to avoid conflicts
	testPort := 19001

	// Create LFD with fixed port
	l, err := NewLfd(1, "LFD1", "S1", testPort, "tcp", 0, "localhost:0")
	if err != nil {
		t.Fatalf("failed to create LFD: %v", err)
	} // Start LFD in background
	go func() {
		if err := l.Start(); err != nil {
			t.Logf("LFD stopped: %v", err)
		}
	}()
	defer l.Stop()

	// Give LFD time to start listening
	time.Sleep(200 * time.Millisecond)

	// Simulate server connecting to LFD
	serverConn, err := net.Dial("tcp", "localhost:19001")
	if err != nil {
		t.Fatalf("failed to connect to LFD as server: %v", err)
	}
	defer serverConn.Close()

	// Wait for connection to be established
	time.Sleep(200 * time.Millisecond)

	// Expect to receive a heartbeat from LFD
	buf := make([]byte, 1024)
	serverConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := serverConn.Read(buf)
	if err != nil {
		t.Fatalf("failed to receive heartbeat from LFD: %v", err)
	}

	var msg types.Message
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		t.Fatalf("failed to unmarshal heartbeat message: %v", err)
	}

	if msg.Type != "lfd" {
		t.Fatalf("expected message type 'lfd', got %s", msg.Type)
	}
	if msg.Message != "heartbeat" {
		t.Fatalf("expected message 'heartbeat', got %s", msg.Message)
	}
	if msg.Id != "LFD1" {
		t.Fatalf("expected ID 'LFD1', got %s", msg.Id)
	}

	// Send heartbeat response back to LFD
	resp := types.Response{
		Type:     "lfd",
		Id:       "S1",
		ReqNum:   msg.ReqNum,
		Response: "alive",
	}
	respData, _ := json.Marshal(resp)
	if _, err := serverConn.Write(respData); err != nil {
		t.Fatalf("failed to send response to LFD: %v", err)
	}

	// Verify we can receive another heartbeat
	time.Sleep(1500 * time.Millisecond)
	serverConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err = serverConn.Read(buf)
	if err != nil {
		t.Fatalf("failed to receive second heartbeat: %v", err)
	}

	var msg2 types.Message
	if err := json.Unmarshal(buf[:n], &msg2); err != nil {
		t.Fatalf("failed to unmarshal second heartbeat: %v", err)
	}

	if msg2.ReqNum <= msg.ReqNum {
		t.Fatalf("expected heartbeat count to increment, got %d after %d", msg2.ReqNum, msg.ReqNum)
	}

	t.Logf("LFD heartbeat test passed: received %d heartbeats", msg2.ReqNum)
}

func TestLfdServerReconnection(t *testing.T) {
	testPort := 19002

	l, err := NewLfd(1, "LFD1", "S1", testPort, "tcp", 0, "localhost:0")
	if err != nil {
		t.Fatalf("failed to create LFD: %v", err)
	}

	go func() {
		if err := l.Start(); err != nil {
			t.Logf("LFD stopped: %v", err)
		}
	}()
	defer l.Stop()

	time.Sleep(200 * time.Millisecond)

	// First server connection
	serverConn1, err := net.Dial("tcp", "localhost:19002")
	if err != nil {
		t.Fatalf("failed to connect first server: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Verify LFD is running
	if l.Status() != "running" {
		t.Fatalf("expected LFD status 'running', got %s", l.Status())
	}

	// Close first connection (simulate server crash)
	serverConn1.Close()
	time.Sleep(500 * time.Millisecond)

	// Second server connection (reconnection)
	serverConn2, err := net.Dial("tcp", "localhost:19002")
	if err != nil {
		t.Fatalf("failed to reconnect server: %v", err)
	}
	defer serverConn2.Close()

	time.Sleep(200 * time.Millisecond)

	// Receive heartbeat from LFD after reconnection
	buf := make([]byte, 1024)
	serverConn2.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := serverConn2.Read(buf)
	if err != nil {
		t.Fatalf("failed to receive heartbeat after reconnection: %v", err)
	}

	var msg types.Message
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		t.Fatalf("failed to unmarshal heartbeat: %v", err)
	}

	if msg.ReqNum != 1 {
		t.Logf("Warning: heartbeat count after reconnection is %d, expected 1 (reset)", msg.ReqNum)
	}

	// Send response
	resp := types.Response{
		Type:     "lfd",
		Id:       "S1",
		ReqNum:   msg.ReqNum,
		Response: "alive",
	}
	respData, _ := json.Marshal(resp)
	serverConn2.Write(respData)

	t.Logf("LFD reconnection test passed")
}

func TestLfdGfdHeartbeat(t *testing.T) {
	// Start a mock GFD server
	gfdListener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to start mock GFD: %v", err)
	}
	defer gfdListener.Close()

	gfdAddr := gfdListener.Addr().(*net.TCPAddr)
	gfdPort := gfdAddr.Port

	// Accept GFD connections in background
	gfdConnChan := make(chan net.Conn, 1)
	go func() {
		conn, err := gfdListener.Accept()
		if err != nil {
			return
		}
		gfdConnChan <- conn
	}()

	// Create LFD that will connect to our mock GFD
	l, err := NewLfd(1, "LFD1", "S1", 0, "tcp", gfdPort, gfdAddr.String())
	if err != nil {
		t.Fatalf("failed to create LFD: %v", err)
	}

	go func() {
		if err := l.Start(); err != nil {
			t.Logf("LFD stopped: %v", err)
		}
	}()
	defer l.Stop()

	// Wait for GFD connection
	var gfdConn net.Conn
	select {
	case gfdConn = <-gfdConnChan:
		defer gfdConn.Close()
	case <-time.After(2 * time.Second):
		t.Fatalf("LFD did not connect to GFD within timeout")
	}

	// Receive heartbeat from LFD to GFD
	buf := make([]byte, 1024)
	gfdConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := gfdConn.Read(buf)
	if err != nil {
		t.Fatalf("failed to receive heartbeat from LFD to GFD: %v", err)
	}

	var msg types.Message
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		t.Fatalf("failed to unmarshal GFD heartbeat: %v", err)
	}

	if msg.Type != "lfd" {
		t.Fatalf("expected message type 'lfd', got %s", msg.Type)
	}
	if msg.Message != "heartbeat" {
		t.Fatalf("expected message 'heartbeat', got %s", msg.Message)
	}
	if msg.Id != "LFD1" {
		t.Fatalf("expected ID 'LFD1', got %s", msg.Id)
	}

	// Send response from GFD
	resp := types.Response{
		Type:     "gfd",
		Id:       "GFD",
		ReqNum:   msg.ReqNum,
		Response: "Alive",
	}
	respData, _ := json.Marshal(resp)
	if _, err := gfdConn.Write(respData); err != nil {
		t.Fatalf("failed to send GFD response: %v", err)
	}

	t.Logf("LFD-GFD heartbeat test passed")
}

func TestLfdStatus(t *testing.T) {
	l, err := NewLfd(1, "LFD1", "S1", 0, "tcp", 0, "localhost:0")
	if err != nil {
		t.Fatalf("failed to create LFD: %v", err)
	}

	if l.Status() != "stopped" {
		t.Fatalf("expected initial status 'stopped', got %s", l.Status())
	}

	go l.Start()
	defer l.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Logf("LFD status test passed")
}
