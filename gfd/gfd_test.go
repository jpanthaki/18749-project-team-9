package gfd

import (
	"18749-team9/types"
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestGfdLifecycle_AddRemoveHeartbeat(t *testing.T) {
	g, err := NewGfd(0, "tcp")
	if err != nil {
		t.Fatalf("failed to create GFD: %v", err)
	}

	g_star := g.(*gfd)

	if err := g.Start(); err != nil {
		t.Fatalf("failed to start GFD: %v", err)
	}
	defer g.Stop()

	addr := g_star.listener.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to connect to GFD: %v", err)
	}
	defer conn.Close()

	send := func(msg types.Message) types.Response {
		data, _ := json.Marshal(msg)
		if _, err := conn.Write(data); err != nil {
			t.Fatalf("failed to send message: %v", err)
		}
		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("failed to read response: %v", err)
		}
		var resp types.Response
		if err := json.Unmarshal(buf[:n], &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		return resp
	}

	// Give server a tiny bit of time to spin up
	time.Sleep(100 * time.Millisecond)

	addMsg := types.Message{
		Type:    "lfd",
		Id:      "S1",
		Message: "add",
		ReqNum:  1,
	}
	resp := send(addMsg)
	if resp.Response != "Added" {
		t.Fatalf("expected Added, got %s", resp.Response)
	}

	hbMsg := types.Message{
		Type:    "lfd",
		Id:      "S1",
		Message: "heartbeat",
		ReqNum:  2,
	}
	resp = send(hbMsg)
	if resp.Response != "Alive" {
		t.Fatalf("expected Alive, got %s", resp.Response)
	}

	rmMsg := types.Message{
		Type:    "lfd",
		Id:      "S1",
		Message: "remove",
		ReqNum:  3,
	}
	resp = send(rmMsg)
	if resp.Response != "Removed" {
		t.Fatalf("expected Removed, got %s", resp.Response)
	}

	time.Sleep(200 * time.Millisecond)
}

func sendMsg(t *testing.T, addr string, msg types.Message) types.Response {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("failed to connect to GFD at %s: %v", addr, err)
	}
	defer conn.Close()

	bytes, _ := json.Marshal(msg)
	if _, err := conn.Write(bytes); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp types.Response
	if err := json.Unmarshal(buf[:n], &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return resp
}

func TestFullGFDScenario(t *testing.T) {
	g, err := NewGfd(0, "tcp")
	if err != nil {
		t.Fatalf("failed to create GFD: %v", err)
	}

	g_star := g.(*gfd)

	if err := g.Start(); err != nil {
		t.Fatalf("failed to start GFD: %v", err)
	}
	defer g.Stop()

	addr := g_star.listener.Addr().String()
	time.Sleep(200 * time.Millisecond)

	for i := 1; i <= 3; i++ {
		//lfdID := i
		replicaID := "S" + string(rune('0'+i))
		msg := types.Message{
			Type:    "lfd",
			Id:      replicaID,
			Message: "add",
			ReqNum:  i,
		}
		resp := sendMsg(t, addr, msg)
		if resp.Response != "Added" {
			t.Fatalf("expected Added for %s, got %s", replicaID, resp.Response)
		}
		time.Sleep(100 * time.Millisecond)
	}

	if g_star.memberCount != 3 {
		t.Fatalf("expected 3 members, got %d", g_star.memberCount)
	}
	for _, id := range []string{"S1", "S2", "S3"} {
		if !g_star.membership[id] {
			t.Fatalf("expected %s to be in membership", id)
		}
	}

	msg := types.Message{
		Type:    "lfd",
		Id:      "S1",
		Message: "remove",
		ReqNum:  10,
	}
	resp := sendMsg(t, addr, msg)
	if resp.Response != "Removed" {
		t.Fatalf("expected Removed, got %s", resp.Response)
	}

	time.Sleep(200 * time.Millisecond)

	if g_star.memberCount != 2 {
		t.Fatalf("expected 2 members after removal, got %d", g_star.memberCount)
	}
	if g_star.membership["S1"] {
		t.Fatalf("expected S1 to be false in membership map")
	}

	for _, id := range []string{"S2", "S3"} {
		if !g_star.membership[id] {
			t.Fatalf("%s should still be active after removal", id)
		}
	}

	t.Logf("GFD scenario test passed: final members = %v", g_star.membership)
}
