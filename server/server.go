package server

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type server struct {
	id       string
	port     int
	protocol string
	listener net.Listener

	state   map[string]int
	isReady bool
	status  string
	msgCh   chan struct {
		id         string
		message    types.Message
		responseCh chan types.Response
	}
	readyCh chan chan bool
	closeCh chan struct{}

	connections map[net.Conn]struct{} // Track active connections

	lfdPort int
	lfdConn net.Conn

	logger *log.Logger
}

func NewServer(id string, port int, protocol string, lfdPort int) (Server, error) {
	s := &server{
		id:       id,
		port:     port,
		protocol: protocol,
		state:    make(map[string]int),
		isReady:  false,
		status:   "stopped",

		msgCh: make(chan struct {
			id         string
			message    types.Message
			responseCh chan types.Response
		}),
		closeCh:     make(chan struct{}),
		connections: make(map[net.Conn]struct{}), // Initialize client map

		lfdPort: lfdPort,

		logger: log.New("Server"),
	}
	return s, nil
}

func (s *server) Start() error {
	l, err := net.Listen(s.protocol, ":"+strconv.Itoa(s.port))
	if err != nil {
		return err
	}
	s.listener = l
	s.status = "running"

	ready := make(chan struct{})
	// Start manager goroutine
	go s.manager()
	go s.listen(ready)
	go s.connectToLFD()

	// Wait for listener to be ready
	<-ready
	s.isReady = true

	return nil
}

func (s *server) Stop() error {
	if s.status != "running" {
		return fmt.Errorf("server not running")
	}
	s.status = "stopped"
	s.isReady = false
	if s.listener != nil {
		s.listener.Close()
	}
	// Close all active connections
	for conn := range s.connections {
		conn.Close()
	}
	if s.lfdConn != nil {
		s.lfdConn.Close()
	}
	close(s.closeCh) // Signal manager goroutine to exit
	return nil
}

func (s *server) Status() string {
	return s.status
}

func (s *server) Ready() bool {
	return s.isReady
}

func (s *server) connectToLFD() {
	for {
		conn, err := net.Dial(s.protocol, ":"+strconv.Itoa(s.lfdPort))
		if err == nil {

			s.lfdConn = conn

			go s.handleConnection(conn)
			return
		}
	}
}

func (s *server) listen(readyCh chan struct{}) {
	close(readyCh)
	defer s.listener.Close()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.connections[conn] = struct{}{} // Add the new connection to the clients map

		go s.handleConnection(conn)
	}
}

func (s *server) manager() {
	for {
		select {
		case msg := <-s.msgCh:
			var resp types.Response
			var logMsg string
			switch msg.message.Type {
			case "client":
				logMsg = fmt.Sprintf("Received <%s, %s, %d, %s>", msg.message.Id, s.id, msg.message.ReqNum, msg.message.Message)
				s.logger.Log(logMsg, "MessageReceived")
				switch msg.message.Message {
				case "Init":
					logMsg = fmt.Sprintf("State = %v before processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.logger.Log(logMsg, "StateBefore")
					s.state[msg.id] = 0
					logMsg = fmt.Sprintf("State = %v after processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.logger.Log(logMsg, "StateAfter")
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("Client: %s Initialized, State: %d", msg.id, s.state[msg.id])}
				case "CountUp":
					logMsg = fmt.Sprintf("State = %v before processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.logger.Log(logMsg, "StateBefore")
					s.state[msg.id]++
					logMsg = fmt.Sprintf("State = %v after processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.logger.Log(logMsg, "StateAfter")
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Up, State: %d}", msg.id, s.state[msg.id])}
				case "CountDown":
					logMsg = fmt.Sprintf("State = %v before processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.logger.Log(logMsg, "StateBefore")
					s.state[msg.id]--
					logMsg = fmt.Sprintf("State = %v after processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.logger.Log(logMsg, "StateAfter")
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Down, State: %d}", msg.id, s.state[msg.id])}
				case "Close":
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Connection closed"}
				default:
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Unknown request"}
				}
				logMsg = fmt.Sprintf("Sending <%s, %s, %d, %s>", resp.Id, s.id, resp.ReqNum, resp.Response)
				s.logger.Log(logMsg, "MessageSent")
				msg.responseCh <- resp
			case "lfd":
				logMsg = fmt.Sprintf("<%d> Received heartbeat from %s", msg.message.ReqNum, msg.message.Id)
				s.logger.Log(logMsg, "HeartbeatReceived")
				resp = types.Response{Type: "lfd", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("%d", msg.message.ReqNum)}
				logMsg = fmt.Sprintf("<%d> Sent heartbeat to %s", resp.ReqNum, resp.Id)
				s.logger.Log(logMsg, "HeartbeatSent")
				msg.responseCh <- resp
			}
		case <-s.closeCh:
			return
		}
	}
}

func (s *server) handleConnection(conn net.Conn) {
	defer conn.Close()
	defer delete(s.connections, conn) // Remove the connection from the clients map when done

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

		request := struct {
			id         string
			message    types.Message
			responseCh chan types.Response
		}{
			id:         msg.Id,
			message:    msg,
			responseCh: make(chan types.Response),
		}

		s.msgCh <- request
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
