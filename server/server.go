package server

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/charmbracelet/log"
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
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
	})
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

		logger: logger,
	}
	return s, nil
}

func (s *server) Start() error {
	l, err := net.Listen(s.protocol, ":"+strconv.Itoa(s.port))
	if err != nil {
		s.logger.Errorf("Error starting listener on server %s: %v", s.id, err)
		return err
	}
	s.listener = l
	s.status = "running"
	s.logger.Infof("Listening on %s", s.listener.Addr().String())

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
	s.logger.Infof("Stopping server %s...", s.id)
	s.status = "stopped"
	s.isReady = false
	if s.listener != nil {
		s.logger.Infof("Closing server %s listener on port %d", s.id, s.port)
		s.listener.Close() // This will cause Accept() to return an error and exit Start()
	}
	// Close all active connections
	for conn := range s.connections {
		s.logger.Infof("Closing connection %s on port %d", conn.RemoteAddr().String(), s.port)
		conn.Close()
	}
	if s.lfdConn != nil {
		s.lfdConn.Close()
	}
	close(s.closeCh) // Signal manager goroutine to exit
	s.logger.Infof("Server %s stopped successfully!", s.id)
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
			s.logger.Infof("Connected to LFD on port %d", s.lfdPort)

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
		s.logger.Infof("Connected to a client with address %s", conn.RemoteAddr().String())

		s.connections[conn] = struct{}{} // Add the new connection to the clients map

		go s.handleConnection(conn)
	}
}

func (s *server) manager() {
	for {
		select {
		case msg := <-s.msgCh:
			var resp types.Response
			switch msg.message.Type {
			case "client":
				switch msg.message.Message {
				case "Init":
					s.logger.Infof("State = %v before processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.state[msg.id] = 0
					s.logger.Infof("State = %v after processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("Client: %s Initialized, State: %d", msg.id, s.state[msg.id])}
				case "CountUp":
					s.logger.Infof("State = %v before processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.state[msg.id]++
					s.logger.Infof("State = %v after processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Up, State: %d}", msg.id, s.state[msg.id])}
				case "CountDown":
					s.logger.Infof("State = %v before processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					s.state[msg.id]--
					s.logger.Infof("State = %v after processing <%s, %s, %d, %s>", s.state, msg.id, s.id, msg.message.ReqNum, msg.message.Message)
					resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Down, State: %d}", msg.id, s.state[msg.id])}
				case "Close":
					resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: "Connection closed"}
				default:
					resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: "Unknown request"}
				}
			case "lfd":
				resp = types.Response{Type: "lfd", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("%d", msg.message.ReqNum)}
			}

			msg.responseCh <- resp
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
			s.logger.Warnf("Error reading from connection: %v", err)
			return
		}
		var msg types.Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			s.logger.Warnf("Error unmarshalling message from connection: %v", err)
			return
		}

		if msg.Type == "client" {
			s.logger.Infof("Received <%s, %s, %d, %s>", msg.Id, s.id, msg.ReqNum, msg.Message)
		} else if msg.Type == "lfd" {
			s.logger.Infof("<%d> Received heartbeat from %s", msg.ReqNum, msg.Id)
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

		if msg.Type == "client" {
			s.logger.Infof("Sending <%s, %s, %d, %s>", response.Id, s.id, response.ReqNum, response.Response)
		} else if msg.Type == "lfd" {
			s.logger.Infof("<%d> Sent heartbeat to %s", response.ReqNum, response.Id)
		}

		respBytes, err := json.Marshal(response)
		if err != nil {
			s.logger.Warnf("Error marshalling response from connection: %v", err)
			return
		}

		_, err = conn.Write(respBytes)
		if err != nil {
			s.logger.Warnf("Error writing to connection: %v", err)
			return
		}
	}
}
