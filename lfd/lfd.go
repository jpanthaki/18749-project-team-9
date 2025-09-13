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
	id                  string
	heartbeat_count     int
	heartbeat_frequency int
	port                int
	protocol            string
	status              string
	conn                net.Conn
	closeCh             chan struct{}
}

func NewLfd(heartbeat_frequency int, id string, port int, protocol string) (Lfd, error) {
	l := &lfd{
		id:                  id,
		port:                port,
		protocol:            protocol,
		status:              "stopped",
		heartbeat_count:     1,
		heartbeat_frequency: heartbeat_frequency,
		closeCh:             make(chan struct{}),
	}
	return l, nil
}

func (l *lfd) Start() error {
	addr := ":" + strconv.Itoa(l.port)
	conn, err := net.Dial(l.protocol, addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	l.conn = conn
	l.status = "running"
	fmt.Printf("[%s] %s connected to server on port %d\n", time.Now().Format("2006-01-02 15:04:05"), l.id, l.port)
	for {
		select {
		case <-l.closeCh:
			return nil
		default:
			err := l.Heartbeat()
			if err != nil {
				l.status = "stopped"
				l.conn.Close()
				return fmt.Errorf("heartbeat failed: %w", err)
			}
			time.Sleep(time.Duration(l.heartbeat_frequency) * time.Second)
		}
	}
}

func (l *lfd) Stop() error {
	close(l.closeCh)
	if l.conn != nil {
		l.conn.Close()
	}
	l.status = "stopped"
	return nil
}

func (l *lfd) Status() string {
	return l.status
}

// Heartbeat sends a single heartbeat message to the server and waits for a response.
func (l *lfd) Heartbeat() error {
	msg := types.Message{
		Type:    "lfd",
		Id:      l.id,
		ReqNum:  l.heartbeat_count,
		Message: "heartbeat",
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}
	_, err = l.conn.Write(data)
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%d] %s sending heartbeat to %s\n", now, l.heartbeat_count, l.id, "S1")
	if err != nil {
		return fmt.Errorf("failed to send heartbeat: %w", err)
	}
	l.conn.SetReadDeadline(time.Now().Add(time.Duration(l.heartbeat_frequency) * time.Second))
	buf := make([]byte, 1024)
	n, err := l.conn.Read(buf)
	if err != nil {
		fmt.Printf("[%s] [%d] %s: Heartbeat to %s failed (%v)\n", now, l.heartbeat_count, l.id, "S1", err)
		return fmt.Errorf("no response from server: %w", err)
	}
	fmt.Printf("[%s] [%d] %s receives heartbeat from %s\n", now, l.heartbeat_count, l.id, "S1")
	var resp types.Response
	err = json.Unmarshal(buf[:n], &resp)
	if err != nil {
		return fmt.Errorf("invalid response from server: %w", err)
	}
	fmt.Printf("Received response: %+v\n", resp)
	l.heartbeat_count++
	return nil
}
