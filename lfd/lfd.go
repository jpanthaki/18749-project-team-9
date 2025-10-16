package lfd

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
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

	gfdPort            int
	gfdConn            net.Conn
	gfdHeartbeatCount  int
	serverRegistered   bool // Track if server has been registered with GFD
	firstHeartbeatDone bool // Track if first heartbeat to server succeeded
}

func NewLfd(heartbeatFrequency int, id string, serverID string, port int, protocol string, gfdPort int) (Lfd, error) {
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
		conn, err := net.Dial(l.protocol, ":"+strconv.Itoa(l.gfdPort))
		if err == nil {
			fmt.Printf("[%s] %s connected to GFD on port %d\n", time.Now().Format("2006-01-02 15:04:05"), l.id, l.gfdPort)
			l.gfdConn = conn
			go l.gfdHeartbeatLoop()
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func (l *lfd) gfdHeartbeatLoop() {
	ticker := time.NewTicker(time.Duration(l.heartbeatFrequency) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-l.closeCh:
			return
		case <-ticker.C:
			err := l.sendGFDHeartbeat()
			if err != nil {
				// GFD connection lost
				if l.gfdConn != nil {
					l.gfdConn.Close()
					l.gfdConn = nil
				}
				// Try to reconnect
				go l.connectToGFD()
				return
			}
		}
	}
}

func (l *lfd) Stop() error {
	close(l.closeCh)
	if l.conn != nil {
		l.conn.Close()
	}
	if l.gfdConn != nil {
		l.gfdConn.Close()
	}
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

	msg := types.Message{
		Type:    "lfd",
		Id:      l.id,
		ReqNum:  l.heartbeatCount,
		Message: "heartbeat",
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
func (l *lfd) sendGFDHeartbeat() error {
	if l.gfdConn == nil {
		return fmt.Errorf("no GFD connection")
	}

	msg := types.Message{
		Type:    "lfd",
		Id:      l.id,
		ReqNum:  l.gfdHeartbeatCount,
		Message: "heartbeat",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal GFD heartbeat: %w", err)
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	_, err = l.gfdConn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send GFD heartbeat: %w", err)
	}
	fmt.Printf("\033[36m[%s] [%d] %s sending heartbeat to GFD\033[0m\n", now, l.gfdHeartbeatCount, l.id)

	l.gfdConn.SetReadDeadline(time.Now().Add(time.Duration(l.heartbeatFrequency) * time.Second))
	buf := make([]byte, 1024)
	n, err := l.gfdConn.Read(buf)
	if err != nil {
		return fmt.Errorf("no response from GFD: %w", err)
	}

	var resp types.Response
	err = json.Unmarshal(buf[:n], &resp)
	if err != nil {
		return fmt.Errorf("invalid response from GFD: %w", err)
	}

	fmt.Printf("\033[36m[%s] [%d] %s received heartbeat ACK from GFD\033[0m\n", now, l.gfdHeartbeatCount, l.id)
	l.gfdHeartbeatCount++
	return nil
}

// notifyGFDAddReplica sends an "add replica" message to GFD after first successful heartbeat
func (l *lfd) notifyGFDAddReplica() {
	if l.gfdConn == nil {
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

	_, err = l.gfdConn.Write(data)
	if err != nil {
		return
	}

	fmt.Printf("\033[33m[%s] Sending to GFD: <%s,GFD,add replica,%s>\033[0m\n", now, l.id, l.serverID)

	// Read response
	l.gfdConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := l.gfdConn.Read(buf)
	if err != nil {
		return
	}

	var resp types.Response
	json.Unmarshal(buf[:n], &resp)
}

// notifyGFDDeleteReplica sends a "delete replica" message to GFD when server dies
func (l *lfd) notifyGFDDeleteReplica() {
	if l.gfdConn == nil {
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

	_, err = l.gfdConn.Write(data)
	if err != nil {
		return
	}

	fmt.Printf("\033[33m[%s] Sending to GFD: <%s,GFD,delete replica,%s>\033[0m\n", now, l.id, l.serverID)

	// Read response
	l.gfdConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1024)
	n, err := l.gfdConn.Read(buf)
	if err != nil {
		return
	}

	var resp types.Response
	json.Unmarshal(buf[:n], &resp)
}
