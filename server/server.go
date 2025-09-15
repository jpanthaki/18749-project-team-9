package server

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"18749-team9/types"
)

type clientMessage struct {
	id         string
	message    types.Message
	responseCh chan types.Response
}

type Server interface {
	Start() error
	Stop() error
	Status() string
	Ready() bool
}

type server struct {
	id       string
	port     int
	protocol string
	listener net.Listener

	state   map[string]int
	isReady bool
	status  string
	msgCh   chan clientMessage
	readyCh chan chan bool
	closeCh chan struct{}

	connections map[net.Conn]struct{} // Track active connections
}

func NewServer(id string, port int, protocol string) (Server, error) {
	s := &server{
		id:       id,
		port:     port,
		protocol: protocol,
		state:    make(map[string]int),
		isReady:  false,
		status:   "stopped",

		msgCh:       make(chan clientMessage),
		closeCh:     make(chan struct{}),
		connections: make(map[net.Conn]struct{}), // Initialize client map
	}
	return s, nil
}

func (s *server) Start() error {
	fmt.Println(s.port)
	l, err := net.Listen(s.protocol, ":"+strconv.Itoa(s.port))
	if err != nil {
		return err
	}
	s.listener = l
	s.status = "running"
	fmt.Printf("Listening on %d\n", s.port)

	ready := make(chan struct{})
	// Start manager goroutine
	go s.manager()
	go s.listen(ready)

	// Wait for listener to be ready
	<-ready
	s.isReady = true

	return nil
}

func (s *server) Stop() error {
	if s.status != "running" {
		return fmt.Errorf("server not running")
	}
	fmt.Println("Stopping server...")
	s.status = "stopped"
	s.isReady = false
	if s.listener != nil {
		s.listener.Close() // This will cause Accept() to return an error and exit Start()
	}
	// Close all active connections
	for conn := range s.connections {
		conn.Close()
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

func (s *server) listen(readyCh chan struct{}) {
	close(readyCh)
	defer s.listener.Close()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		fmt.Println("connected to a client")

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
					s.state[msg.id] = 0
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("Client: %s Initialized, State: %d", msg.id, s.state[msg.id])}
				case "CountUp":
					s.state[msg.id]++
					fmt.Println("State after CountUp:", s.state)
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Up, State: %d}", msg.id, s.state[msg.id])}
				case "CountDown":
					s.state[msg.id]--
					fmt.Println("State after CountDown:", s.state)
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Down, State: %d}", msg.id, s.state[msg.id])}
				case "Close":
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Connection closed"}
				default:
					resp = types.Response{Type: "client", Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Unknown request"}
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
			return
		}
		var msg types.Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			return
		}
		fmt.Println("Received message:", msg)

		request := clientMessage{
			id:         msg.Id,
			message:    msg,
			responseCh: make(chan types.Response),
		}
		fmt.Printf("Request: %+v\n", msg)
		s.msgCh <- request
		response := <-request.responseCh
		fmt.Printf("Response: %+v\n", response)

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
