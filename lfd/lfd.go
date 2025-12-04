package lfd

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

type Lfd interface {
	Start() error
	Stop() error
	Status() string
}

type lfd struct {
	id                 string
	serverID           string // ID of the server replica being monitored
	heartbeatCount     int
	heartbeatFrequency int
	port               int
	protocol           string
	status             string
	conn               net.Conn
	closeCh            chan struct{}
	writeCh            chan types.Message
	sendPromotion      bool // Send promotion flag
	promotionPayload   json.RawMessage

	gfdPort            int
	gfdAddr            string
	gfdConn            net.Conn
	gfdConnMu          sync.Mutex // Protects gfdConn access
	sendPromotionMu    sync.Mutex // Protects sendPromotion access
	gfdHeartbeatCount  int
	serverRegistered   bool // Track if server has been registered with GFD
	firstHeartbeatDone bool // Track if first heartbeat to server succeeded
}

func NewLfd(heartbeatFrequency int, id string, serverID string, port int, protocol string, gfdPort int, gfdAddr string) (Lfd, error) {
	l := &lfd{
		id:                 id,
		serverID:           serverID,
		port:               port,
		protocol:           protocol,
		status:             "stopped",
		heartbeatCount:     1,
		heartbeatFrequency: heartbeatFrequency,
		closeCh:            make(chan struct{}),
		gfdPort:            gfdPort,
		gfdAddr:            gfdAddr,
		gfdHeartbeatCount:  1,
		serverRegistered:   false,
		firstHeartbeatDone: false,
	}
	return l, nil
}

func (l *lfd) Start() error {
	addr := ":" + strconv.Itoa(l.port)
	listener, err := net.Listen(l.protocol, addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	defer listener.Close()

	fmt.Printf("[%s] %s listening on port %d\n", time.Now().Format("2006-01-02 15:04:05"), l.id, l.port)

	// Connection routines
	go l.connectToGFD()
	go l.acceptServerConnections(listener)

	// Main loop
	<-l.closeCh
	return nil
}

func (l *lfd) acceptServerConnections(listener net.Listener) {
	for {
		select {
		case <-l.closeCh:
			return
		default:
			conn, err := listener.Accept()
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}

			if l.conn != nil {
				l.conn.Close()
			}
			l.conn = conn
			l.status = "running"
			l.heartbeatCount = 1
			l.firstHeartbeatDone = false
			l.serverRegistered = false

			fmt.Printf("[%s] %s connected to server on port %d\n", time.Now().Format("2006-01-02 15:04:05"), l.id, l.port)

			// Start heartbeating this connection
			go l.heartbeatLoop()
		}
	}
}

func (l *lfd) heartbeatLoop() {
	ticker := time.NewTicker(time.Duration(l.heartbeatFrequency) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.closeCh:
			return
		case <-ticker.C:
			err := l.Heartbeat()
			if err != nil {
				// Server connection lost - notify GFD and wait for reconnection
				now := time.Now().Format("2006-01-02 15:04:05")
				fmt.Printf("\033[31m[%s] %s has died.\033[0m\n", now, l.serverID)
				l.status = "server_down"

				// Send delete replica message to GFD
				if l.serverRegistered {
					l.notifyGFDDeleteReplica()
					l.serverRegistered = false
				}

				if l.conn != nil {
					l.conn.Close()
					l.conn = nil
				}
				return
			}

			// After first successful heartbeat, register server with GFD
			if !l.serverRegistered && l.firstHeartbeatDone {
				l.notifyGFDAddReplica()
				l.serverRegistered = true
			}
		}
	}
}

func (l *lfd) connectToGFD() {
	for {
		conn, err := net.Dial(l.protocol, l.gfdAddr)
		if err == nil {
			fmt.Printf("[%s] %s connected to GFD at %s\n", time.Now().Format("2006-01-02 15:04:05"), l.id, l.gfdAddr)
			l.gfdConnMu.Lock()
			l.gfdConn = conn
			l.gfdConnMu.Unlock()
			go l.gfdLoop()
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func (l *lfd) gfdLoop() {
	ticker := time.NewTicker(time.Duration(l.heartbeatFrequency) * time.Second)
	defer ticker.Stop()

	// Get GFD connection
	l.gfdConnMu.Lock()
	conn := l.gfdConn
	l.gfdConnMu.Unlock()

	if conn == nil {
		return
	}

	// Channel to signal when we're expecting a heartbeat response
	heartbeatPending := false

	for {
		select {
		case <-l.closeCh:
			return
		case <-ticker.C:
			// Send heartbeat
			msg := types.Message{
				Type:    "lfd",
				Id:      l.id,
				ReqNum:  l.gfdHeartbeatCount,
				Message: "heartbeat",
			}
			data, err := json.Marshal(msg)
			if err != nil {
				return
			}

			_, err = conn.Write(data)
			if err != nil {
				l.handleGFDConnectionLost()
				return
			}
			fmt.Printf("\033[36m[%s] [%d] %s sending heartbeat to GFD\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), l.gfdHeartbeatCount, l.id)
			heartbeatPending = true
			l.gfdHeartbeatCount++

		default:
			// Check for incoming messages from GFD
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout is expected, continue loop
					continue
				}
				// Real error - connection lost
				l.handleGFDConnectionLost()
				return
			}

			var msg types.Message
			err = json.Unmarshal(buf[:n], &msg)
			if err == nil {
				// Check if this is a promotion message from RM (via GFD)
				if msg.Type == "rm" && msg.Message == "Promote" {
					fmt.Printf("\033[32m[%s] %s received promotion from GFD\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), l.id)
					l.sendPromotionMu.Lock()
					l.sendPromotion = true
					l.promotionPayload, _ = json.Marshal(&msg)
					l.sendPromotionMu.Unlock()
					//l.forwardPromotionToServer()
					continue
				}
			}

			// Try to parse as response (for heartbeat ACK)
			var resp types.Response
			err = json.Unmarshal(buf[:n], &resp)
			if err == nil && heartbeatPending {
				fmt.Printf("\033[36m[%s] [%d] %s received heartbeat ACK from GFD\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), l.gfdHeartbeatCount-1, l.id)
				heartbeatPending = false
			}
		}
	}
}

func (l *lfd) handleGFDConnectionLost() {
	l.gfdConnMu.Lock()
	if l.gfdConn != nil {
		l.gfdConn.Close()
		l.gfdConn = nil
	}
	l.gfdConnMu.Unlock()
	// Try to reconnect
	go l.connectToGFD()
}

func (l *lfd) Stop() error {
	close(l.closeCh)
	if l.conn != nil {
		l.conn.Close()
	}
	l.gfdConnMu.Lock()
	if l.gfdConn != nil {
		l.gfdConn.Close()
	}
	l.gfdConnMu.Unlock()
	l.status = "stopped"
	return nil
}

func (l *lfd) Status() string {
	return l.status
}

// Heartbeat sends a single heartbeat message to the server and waits for a response.
func (l *lfd) Heartbeat() error {
	if l.conn == nil {
		return fmt.Errorf("no server connection")
	}

	var payload json.RawMessage
	l.sendPromotionMu.Lock()
	if l.sendPromotion {
		payload = l.promotionPayload
		l.promotionPayload = nil
		l.sendPromotion = false
	} else {
		payload = nil
	}
	l.sendPromotionMu.Unlock()

	msg := types.Message{
		Type:    "lfd",
		Id:      l.id,
		ReqNum:  l.heartbeatCount,
		Message: "heartbeat",
		Payload: payload,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	_, err = l.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	fmt.Printf("\033[36m[%s] [%d] %s sending heartbeat to %s\033[0m\n", now, l.heartbeatCount, l.id, l.serverID)

	l.conn.SetReadDeadline(time.Now().Add(time.Duration(l.heartbeatFrequency) * time.Second))
	buf := make([]byte, 1024)
	n, err := l.conn.Read(buf)
	if err != nil {
		return fmt.Errorf("no response from server: %w", err)
	}

	var resp types.Response
	err = json.Unmarshal(buf[:n], &resp)
	if err != nil {
		return fmt.Errorf("invalid response from server: %w", err)
	}

	l.heartbeatCount++
	l.firstHeartbeatDone = true
	return nil
}

// sendGFDHeartbeat sends a heartbeat to the GFD
// notifyGFDAddReplica sends an "add replica" message to GFD after first successful heartbeat
func (l *lfd) notifyGFDAddReplica() {
	l.gfdConnMu.Lock()
	conn := l.gfdConn
	l.gfdConnMu.Unlock()

	if conn == nil {
		return
	}

	msg := types.Message{
		Type:    "lfd",
		Id:      l.serverID,
		ReqNum:  0,
		Message: "add",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	_, err = conn.Write(data)
	if err != nil {
		return
	}

	fmt.Printf("\033[33m[%s] Sending to GFD: <%s,GFD,add replica,%s>\033[0m\n", now, l.id, l.serverID)

	// Read response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	var resp types.Response
	json.Unmarshal(buf[:n], &resp)
}

// notifyGFDDeleteReplica sends a "delete replica" message to GFD when server dies
func (l *lfd) notifyGFDDeleteReplica() {
	l.gfdConnMu.Lock()
	conn := l.gfdConn
	l.gfdConnMu.Unlock()

	if conn == nil {
		return
	}

	msg := types.Message{
		Type:    "lfd",
		Id:      l.serverID,
		ReqNum:  0,
		Message: "remove",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	_, err = conn.Write(data)
	if err != nil {
		return
	}

	fmt.Printf("\033[33m[%s] Sending to GFD: <%s,GFD,delete replica,%s>\033[0m\n", now, l.id, l.serverID)

	// Read response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	var resp types.Response
	json.Unmarshal(buf[:n], &resp)
}

// listenFromGFD listens for asynchronous messages from GFD (like promotion notifications)
// forwardPromotionToServer forwards the promotion message to the server
func (l *lfd) forwardPromotionToServer() {
	if l.conn == nil {
		return
	}

	msg := types.Message{
		Type:    "rm", // Changed from "lfd" to "rm" to reuse handleRMMessage
		Id:      l.serverID,
		Message: "Promote", // Changed from "promote" to "Promote" to match handleRMMessage
		ReqNum:  0,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_, err = l.conn.Write(data)
	if err != nil {
		return
	}

	fmt.Printf("\033[32m[%s] %s forwarded promotion to server %s\033[0m\n", time.Now().Format("2006-01-02 15:04:05"), l.id, l.serverID)
}
