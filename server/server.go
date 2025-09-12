package server

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type ClientMessage struct {
	id         string
	message    Message
	responseCh chan Response
}

type Server interface {
	Start() error
	Stop() error
	Status() string
}

type server struct {
	id       string
	port     int
	protocol string
	listener net.Listener

	state    int
	isReady  bool
	status   string
	msgCh    chan ClientMessage
	closeCh  chan struct{}
	statusCh chan (chan string)

	connections map[net.Conn]struct{} // Track active connections
}

func NewServer(id string, port int, protocol string) (Server, error) {
	s := &server{
		id:       id,
		port:     port,
		protocol: protocol,
		state:    0,
		isReady:  false,
		status:   "stopped",

		msgCh:       make(chan ClientMessage),
		closeCh:     make(chan struct{}),
		statusCh:    make(chan (chan string)),
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
	s.isReady = true
	fmt.Printf("Listening on %d\n", s.port)
	defer l.Close()

	go s.manager()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		fmt.Println("connected to a client")

		s.connections[conn] = struct{}{} // Add the new connection to the clients map

		go s.handleConnection(conn)
	}
}

func (s *server) Stop() error {
	if s.status != "running" {
		return fmt.Errorf("server not running")
	}
	fmt.Println("Stopping server...")
	s.status = "stopped"
	if s.listener != nil {
		s.listener.Close() // This will cause Accept() to return an error and exit Start()
	}
	// Close all active client connections
	for conn := range s.connections {
		conn.Close()
	}
	close(s.closeCh) // Signal manager goroutine to exit
	return nil
}

func (s *server) Status() string {
	responseCh := make(chan string)
	s.statusCh <- responseCh
	return <-responseCh
}

func (s *server) manager() {
	for {
		select {
		case msg := <-s.msgCh:
			var resp Response
			switch msg.message.Type {
			case "client":
				switch msg.message.Message {
				case "Init":
					resp = Response{Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Initialized"}
				case "CountUp":
					s.state++
					resp = Response{Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s, State: %d}", msg.id, s.state)}
				case "CountDown":
					s.state--
					resp = Response{Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s, State: %d}", msg.id, s.state)}
				case "Close":
					resp = Response{Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Connection closed"}
				default:
					resp = Response{Id: msg.id, ReqNum: msg.message.ReqNum, Response: "Unknown request"}
				}
			case "lfd":
				resp = Response{Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("%d", msg.message.ReqNum)}
			}

			msg.responseCh <- resp
		case statusRespCh := <-s.statusCh:
			statusRespCh <- s.status
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
		var msg Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			return
		}
		fmt.Println("Received message:", msg)

		request := ClientMessage{
			id:         msg.Id,
			message:    msg,
			responseCh: make(chan Response),
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
