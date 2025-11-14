package passive

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"reflect"
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
	srv, err := NewServer("S1", 0, "tcp", 0, 10000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server in a goroutine since Start blocks
	go func() {
		err := srv.Start(true)
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
	srv, err := NewServer("S1", 0, "tcp", 0, 10000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	_ = srv.Start(true)

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
	srv, err := NewServer("S1", 0, "tcp", 9090, 10000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	go func() {
		err := srv.Start(true)
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

func TestSingleLeaderPromotion(t *testing.T) {
	peers := map[string]string{
		"S2": "123.123.123.123:1234",
		"S3": "123.123.123.123:1234",
	}
	srv, err := NewServer("S1", 0, "tcp", 0, 2000, peers)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	_ = srv.Start(false)

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

	msg := types.Message{
		Type:    "rm",
		Id:      "RM",
		ReqNum:  0,
		Message: "Promote",
	}
	var conn net.Conn
	if conn, err = net.Dial("tcp", fmt.Sprintf(":%d", port)); err != nil {
		t.Fatalf("Dial to server failed...")
	}

	bytes, _ := json.Marshal(&msg)

	conn.Write(bytes)

	time.Sleep(10 * time.Millisecond)

	if !serverImpl.isLeader {
		t.Fatalf("Leader promotion failed...")
	}

	err = srv.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}

func newTestCluster(t *testing.T) (Server, Server, Server) {
	peers := map[string]string{
		"S1": "127.0.0.1:10001",
		"S2": "127.0.0.1:10002",
		"S3": "127.0.0.1:10003",
	}

	s1, _ := NewServer("S1", 10001, "tcp", 5001, 2000, peers)
	s2, _ := NewServer("S2", 10002, "tcp", 5002, 2000, peers)
	s3, _ := NewServer("S3", 10003, "tcp", 5003, 2000, peers)

	go s1.Start(true)
	go s2.Start(false)
	go s3.Start(false)

	time.Sleep(1 * time.Second)
	return s1, s2, s3
}

func teardownCluster(s1, s2, s3 Server) {
	_ = s1.Stop()
	_ = s2.Stop()
	_ = s3.Stop()
}

func sendClientReq(t *testing.T, addr string, msg types.Message) *types.Response {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("client failed to connect to %s: %v", addr, err)
	}
	defer conn.Close()

	data, _ := json.Marshal(msg)
	_, err = conn.Write(data)
	if err != nil {
		t.Fatalf("client write failed: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("client read failed: %v", err)
	}

	var resp types.Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("response unmarshal failed: %v", err)
	}
	return &resp
}

func sendAll(t *testing.T, msg types.Message) types.Response {
	addrs := map[string]string{
		"s1Addr": "127.0.0.1:10001",
		"s2Addr": "127.0.0.1:10002",
		"s3Addr": "127.0.0.1:10003",
	}

	respCh := make(chan types.Response)

	for _, addr := range addrs {
		go func(addr string) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return
			}

			bytes, err := json.Marshal(&msg)
			if err != nil {
				return
			}

			conn.Write(bytes)

			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			var resp types.Response
			json.Unmarshal(buf[:n], &resp)

			fmt.Println(resp)

			respCh <- resp
		}(addr)
	}

	select {
	case resp := <-respCh:
		return resp
	}

}

func TestCheckpointPropagation(t *testing.T) {
	s1, s2, s3 := newTestCluster(t)
	defer teardownCluster(s1, s2, s3)

	s1Impl := s1.(*server)
	s2Impl := s2.(*server)
	s3Impl := s3.(*server)

	msg := types.Message{Type: "client", Id: "C1", ReqNum: 1, Message: "Init"}
	resp := sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Initialized") {
		t.Fatalf("leader did not respond correctly, got: %v", resp.Response)
	}

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 2, Message: "CountUp"}
	_ = sendClientReq(t, "127.0.0.1:10001", msg)

	time.Sleep(5 * time.Second) // allow checkpoints to propagate

	if !reflect.DeepEqual(s1Impl.state, s2Impl.state) {
		t.Fatalf("checkpoint mismatch: S1 %v vs S2 %v", s1Impl.state, s2Impl.state)
	}
	if !reflect.DeepEqual(s1Impl.state, s3Impl.state) {
		t.Fatalf("checkpoint mismatch: S1 %v vs S3 %v", s1Impl.state, s3Impl.state)
	}
}

func TestClientInteraction(t *testing.T) {
	s1, s2, s3 := newTestCluster(t)
	defer teardownCluster(s1, s2, s3)

	msg := types.Message{Type: "client", Id: "C1", ReqNum: 1, Message: "Init"}

	// leader should respond
	resp := sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Initialized") {
		t.Fatalf("leader did not respond, got: %v", resp.Response)
	}

	// backups should not respond
	for _, addr := range []string{"127.0.0.1:10002", "127.0.0.1:10003"} {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("client connect to %s failed: %v", addr, err)
		}
		data, _ := json.Marshal(msg)
		_, _ = conn.Write(data)

		// short read timeout
		conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		buf := make([]byte, 512)
		n, err := conn.Read(buf)
		conn.Close()
		if err == nil && n > 0 {
			t.Fatalf("backup %s responded unexpectedly: %s", addr, string(buf[:n]))
		}
	}
}

func TestReplicaFailure(t *testing.T) {
	s1, s2, s3 := newTestCluster(t)
	defer teardownCluster(s1, s2, s3)

	s1Impl := s1.(*server)

	s3Impl := s3.(*server)

	// stop a backup
	_ = s2.Stop()
	time.Sleep(500 * time.Millisecond)

	// client still talks to leader
	msg := types.Message{Type: "client", Id: "C1", ReqNum: 1, Message: "Init"}
	resp := sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Initialized") {
		t.Fatalf("leader failed to handle request after backup stopped, got: %v", resp.Response)
	}

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 2, Message: "CountUp"}
	_ = sendClientReq(t, "127.0.0.1:10001", msg)

	time.Sleep(2 * time.Second)

	if !reflect.DeepEqual(s1Impl.state, s3Impl.state) {
		t.Fatalf("leader and surviving backup inconsistent: S1 %v vs S3 %v", s1Impl.state, s3Impl.state)
	}
}

func Test2ReplicaFailure(t *testing.T) {
	s1, s2, s3 := newTestCluster(t)
	defer teardownCluster(s1, s2, s3)

	s1Impl := s1.(*server)

	// stop a backup
	_ = s2.Stop()
	time.Sleep(500 * time.Millisecond)

	//stop second backup
	_ = s3.Stop()
	time.Sleep(500 * time.Millisecond)

	// client still talks to leader
	msg := types.Message{Type: "client", Id: "C1", ReqNum: 1, Message: "Init"}
	resp := sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Initialized") {
		t.Fatalf("leader failed to handle request after backup stopped, got: %v", resp.Response)
	}

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 2, Message: "CountUp"}
	resp = sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Counted Up") {
		t.Fatalf("leader failed CountUp after both backups stopped: %v", resp.Response)
	}

	// --- Confirm state actually changed on leader ---
	if s1Impl.state["C1"] != 1 {
		t.Fatalf("leader state incorrect after CountUp, expected 1 got %d", s1Impl.state["C1"])
	}
}

func TestPrimaryFailureWithLeaderPromotion(t *testing.T) {
	s1, s2, s3 := newTestCluster(t)
	defer teardownCluster(s1, s2, s3)

	s1Impl := s1.(*server)
	s2Impl := s2.(*server)

	time.Sleep(500 * time.Millisecond)

	// client still talks to leader
	msg := types.Message{Type: "client", Id: "C1", ReqNum: 1, Message: "Init"}
	resp := sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Initialized") {
		t.Fatalf("leader failed to handle request, got: %v", resp.Response)
	}

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 2, Message: "CountUp"}
	resp = sendClientReq(t, "127.0.0.1:10001", msg)
	if !strings.Contains(resp.Response, "Counted Up") {
		t.Fatalf("leader failed CountUp: %v", resp.Response)
	}

	// --- Confirm state actually changed on leader ---
	if s1Impl.state["C1"] != 1 {
		t.Fatalf("leader state incorrect after CountUp, expected 1 got %d", s1Impl.state["C1"])
	}

	//allow some time for checkpointing
	time.Sleep(6 * time.Second)

	//now kill the leader...

	_ = s1.Stop()
	time.Sleep(500 * time.Millisecond)

	//and promote S2...
	rmMsg := types.Message{
		Type:    "rm",
		Id:      "RM",
		ReqNum:  0,
		Message: "Promote",
	}

	var conn net.Conn
	var err error
	if conn, err = net.Dial("tcp", "127.0.0.1:10002"); err != nil {
		t.Fatalf("Dial to server failed...")
	}

	bytes, _ := json.Marshal(&rmMsg)

	conn.Write(bytes)

	time.Sleep(10 * time.Millisecond)

	//now check if S2 is the leader:
	if !s2Impl.isLeader {
		t.Fatalf("S2 was not elected leader")
	}

	//wait for checkpoints...
	time.Sleep(6 * time.Second)

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 3, Message: "CountUp"}
	resp = sendClientReq(t, "127.0.0.1:10002", msg)
	if !strings.Contains(resp.Response, "Counted Up") {
		t.Fatalf("leader failed CountUp: %v", resp.Response)
	}

	// --- Confirm state actually changed on leader ---
	if s2Impl.state["C1"] != 2 {
		t.Fatalf("leader state incorrect after CountUp, expected 2 got %d", s2Impl.state["C1"])
	}
}

func TestReplicaFailAndRecover(t *testing.T) {
	s1, s2, s3 := newTestCluster(t)
	defer teardownCluster(s1, s2, s3)

	s1Impl := s1.(*server)
	s2Impl := s2.(*server)
	s3Impl := s3.(*server)

	time.Sleep(500 * time.Millisecond)

	// client still talks to leader
	msg := types.Message{Type: "client", Id: "C1", ReqNum: 1, Message: "Init"}
	resp := sendAll(t, msg)
	if !strings.Contains(resp.Response, "Initialized") {
		t.Fatalf("leader failed to handle request, got: %v", resp.Response)
	}

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 2, Message: "CountUp"}
	resp = sendAll(t, msg)
	if !strings.Contains(resp.Response, "Counted Up") {
		t.Fatalf("leader failed CountUp: %v", resp.Response)
	}

	// --- Confirm state actually changed on leader ---
	if s1Impl.state["C1"] != 1 {
		t.Fatalf("leader state incorrect after CountUp, expected 1 got %d", s1Impl.state["C1"])
	}

	//allow some time for checkpointing
	time.Sleep(6 * time.Second)

	//confirm state changes on replicas

	if s2Impl.state["C1"] != 1 {
		t.Fatalf("replica state incorrect after checkpoint, expected 1 got %d", s2Impl.state["C1"])
	}

	if s3Impl.state["C1"] != 1 {
		t.Fatalf("replica state incorrect after checkpoint, expected 1 got %d", s3Impl.state["C1"])
	}

	//now kill a replica
	fmt.Println("Killing S2...")
	_ = s2.Stop

	time.Sleep(4000 * time.Millisecond)

	fmt.Println("S2 dead...")

	//try and send another message

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 3, Message: "CountUp"}
	resp = sendAll(t, msg)
	if !strings.Contains(resp.Response, "Counted Up") {
		t.Fatalf("leader failed CountUp: %v", resp.Response)
	}

	//confirm leader state...
	if s1Impl.state["C1"] != 2 {
		t.Fatalf("leader state incorrect after CountUp, expected 2 got %d", s1Impl.state["C1"])
	}

	//sleep for checkpoint

	time.Sleep(6 * time.Second)

	if s3Impl.state["C1"] != 2 {
		t.Fatalf("replica state incorrect after checkpoint, expected 2 got %d", s3Impl.state["C1"])
	}

	time.Sleep(500 * time.Millisecond)

	//now bring s2 back to life...

	go s2.Start(false)

	//sleep for startup and checkpoints
	time.Sleep(6 * time.Second)

	if s2Impl.state["C1"] != 2 {
		t.Fatalf("replica state incorrect after recovery, expected 2 got %d", s3Impl.state["C1"])
	}

	time.Sleep(500 * time.Millisecond)

	//send another message for good measure...

	msg = types.Message{Type: "client", Id: "C1", ReqNum: 4, Message: "CountUp"}
	resp = sendAll(t, msg)
	if !strings.Contains(resp.Response, "Counted Up") {
		t.Fatalf("leader failed CountUp: %v", resp.Response)
	}

	if s1Impl.state["C1"] != 3 {
		t.Fatalf("leader state incorrect, expected 3 got %d", s1Impl.state["C1"])
	}

	time.Sleep(6 * time.Second)

	if s2Impl.state["C1"] != 3 {
		t.Fatalf("replica state incorrect after checkpoint, expected 3 got %d", s2Impl.state["C1"])
	}

	if s3Impl.state["C1"] != 3 {
		t.Fatalf("replica state incorrect after checkpoint, expected 3 got %d", s3Impl.state["C1"])
	}
}
