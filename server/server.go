package server

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
)

type Message struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type ClientMessage struct {
	id         string
	message    Message
	responseCh chan Response
}

type Response struct {
	Id       string `json:"id"`
	Response string `json:"response"`
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

	state    string
	status   string
	msgCh    chan ClientMessage
	closeCh  chan struct{}
	statusCh chan (chan string)

	clients map[net.Conn]struct{} // Track active client connections
}

func NewServer(id string, port int, protocol string) (Server, error) {
	s := &server{
		id:       id,
		port:     port,
		protocol: protocol,
		state:    "Hello World!",
		status:   "stopped",

		msgCh:    make(chan ClientMessage),
		closeCh:  make(chan struct{}),
		statusCh: make(chan (chan string)),
		clients:  make(map[net.Conn]struct{}), // Initialize client map
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
	defer l.Close()

	go s.manager()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		fmt.Println("connected to a client")

		s.clients[conn] = struct{}{} // Add the new connection to the clients map

		go s.handleClient(conn)
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
	for conn := range s.clients {
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
			switch msg.message.Message {
			case "Init":
				resp = Response{Id: msg.id, Response: "Initialized"}
			case "Heartbeat":
				resp = Response{Id: msg.id, Response: "Alive"}
			case "Get":
				resp = Response{Id: msg.id, Response: s.state}
			case "Close":
				resp = Response{Id: msg.id, Response: "Connection closed"}
			default:
				resp = Response{Id: msg.id, Response: "Unknown request"}
			}
			msg.responseCh <- resp
		case statusRespCh := <-s.statusCh:
			statusRespCh <- s.status
		case <-s.closeCh:
			return
		}
	}
}

func (s *server) handleClient(conn net.Conn) {
	defer conn.Close()
	defer delete(s.clients, conn) // Remove the connection from the clients map when done

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
