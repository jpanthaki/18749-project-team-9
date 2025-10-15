package gfd

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
)

type gfd struct {
	port     int
	protocol string
	listener net.Listener

	membership  map[string]bool
	memberCount int

	connections map[net.Conn]struct{}

	msgCh chan struct {
		id         string
		message    types.Message
		responseCh chan types.Response
	}
	closeCh chan struct{}
}

func NewGfd(port int, protocol string) (*gfd, error) {
	g := &gfd{
		port:        port,
		protocol:    protocol,
		membership:  make(map[string]bool),
		memberCount: 0,
		connections: make(map[net.Conn]struct{}),
		msgCh: make(chan struct {
			id         string
			message    types.Message
			responseCh chan types.Response
		}),
		closeCh: make(chan struct{}),
	}
	return g, nil
}

func (g *gfd) Start() error {
	l, err := net.Listen(g.protocol, fmt.Sprintf(":%d", g.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", g.port)
	}

	g.listener = l

	ready := make(chan struct{})
	go g.manager()
	go g.listen(ready)

	<-ready
	return nil
}

func (g *gfd) Stop() error {
	if g.listener != nil {
		g.listener.Close()
	}

	for conn := range g.connections {
		conn.Close()
	}

	close(g.closeCh)

	return nil
}

func (g *gfd) manager() {
	for {
		select {
		case msg := <-g.msgCh:
			var resp types.Response
			switch msg.message.Message {
			case "add":
				g.membership[msg.id] = true
				g.memberCount++
				resp = types.Response{Type: "gfd", Id: msg.id, ReqNum: g.memberCount, Response: "Added"}
			case "remove":
				g.membership[msg.id] = false
				g.memberCount--
				resp = types.Response{Type: "gfd", Id: msg.id, ReqNum: g.memberCount, Response: "Removed"}
			case "heartbeat":
				resp = types.Response{Type: "gfd", Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Alive"}
			}
			msg.responseCh <- resp
		case <-g.closeCh:
			return
		}

	}
}

func (g *gfd) listen(readyCh chan struct{}) {
	close(readyCh)
	defer g.listener.Close()
	for {
		conn, err := g.listener.Accept()
		if err != nil {
			return
		}
		g.connections[conn] = struct{}{}

		go g.handleConnection(conn)
	}
}

func (g *gfd) handleConnection(conn net.Conn) {
	defer conn.Close()
	defer delete(g.connections, conn)

	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}

		var msg types.Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			return
		}

		if msg.Type != "lfd" {
			continue
		}

		request := struct {
			id         string
			message    types.Message
			responseCh chan types.Response
		}{
			id:         msg.Id,
			message:    msg,
			responseCh: make(chan types.Response),
		}

		g.msgCh <- request
		response := <-request.responseCh

		respBytes, err := json.Marshal(response)
		if err != nil {
			return
		}

		_, err = conn.Write(respBytes)
		if err != nil {
			return
		}
	}
}
