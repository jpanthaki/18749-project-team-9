package server

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"
)

func NewServer(id string, port int, protocol string, lfdPort int) (Server, error) { //, replicationMode string, isLeader bool, checkpointFreq int, peers map[string]string
	s := &server{
		id:       id,
		port:     port,
		protocol: protocol,
		state:    make(map[string]int),
		isReady:  false,
		status:   "stopped",

		replicationMode: "active",
		isLeader:        false,
		checkpointFreq:  2000,
		checkpointCount: 0,
		checkpointCh:    make(chan types.Checkpoint),
		peers:           make(map[string]string),

		msgCh:           make(chan internalMessage),
		closeCh:         make(chan struct{}),
		connections:     make(map[net.Conn]struct{}), // Initialize client map
		peerConnections: make(map[string]net.Conn),

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
		err := s.listener.Close()
		if err != nil {
			return err
		}
	}
	// Close all active connections
	for conn := range s.connections {
		_ = conn.Close()
	}
	if s.lfdConn != nil {
		_ = s.lfdConn.Close()
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

type server struct {
	id       string
	port     int
	protocol string
	listener net.Listener

	replicationMode string
	isLeader        bool
	checkpointFreq  int
	checkpointCount int
	checkpointCh    chan types.Checkpoint
	peers           map[string]string

	state   map[string]int
	isReady bool
	status  string
	msgCh   chan internalMessage
	readyCh chan chan bool
	closeCh chan struct{}

	connections     map[net.Conn]struct{} // Track active connections
	peerConnections map[string]net.Conn   //Track other servers

	lfdPort int
	lfdConn net.Conn

	logger *log.Logger
}

type internalMessage struct {
	id         string
	message    types.Message
	responseCh chan types.Response
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
	defer func(listener net.Listener) {
		_ = listener.Close()
	}(s.listener)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.connections[conn] = struct{}{} // Add the new connection to the clients map

		go s.handleConnection(conn)
	}
}

func (s *server) connectToPeers() {
	for {
		for id, addr := range s.peers {
			if s.id[len(s.id)-1] < id[len(id)-1] {
				if _, ok := s.peerConnections[id]; !ok {
					conn, err := net.Dial(s.protocol, addr)
					if err == nil {
						s.peerConnections[id] = conn //TODO need to handle concurrency with this structure here...
						go s.handleConnection(conn)
					}
				}
			}
		}
	}
}

func (s *server) manager() {
	ticker := time.NewTicker(time.Duration(s.checkpointFreq) * time.Millisecond)
	for {
		select {
		case msg := <-s.msgCh:
			var resp types.Response
			switch msg.message.Type {
			case "client":
				if s.replicationMode == "active" || (s.replicationMode == "passive" && s.isLeader) {
					s.logReceived(msg.message)
					s.handleClientMessage(msg, &resp)
					s.logSent(resp)
					msg.responseCh <- resp
				}

			case "lfd":
				s.logHeartbeatReceived(msg.message)
				resp = types.Response{Type: "lfd", Id: msg.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("%d", msg.message.ReqNum)}
				s.logHeartbeatSent(resp)
				msg.responseCh <- resp
			//TODO need to handle case "replica" messages here (these will either be initial connection requests or checkpoints.)

			case "replica":

			}
		case <-s.closeCh:
			return
		case <-ticker.C:
			//TODO need to handle sending of checkpoints if we're passive and the leader.
		}
	}
}

func (s *server) handleClientMessage(msg internalMessage, resp *types.Response) {
	switch msg.message.Message {
	case "Init":
		s.logBefore(msg.message)
		s.state[msg.id] = 0
		s.logAfter(msg.message)
		*resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("Client: %s Initialized, State: %d", msg.id, s.state[msg.id])}

	case "CountUp":
		s.logBefore(msg.message)
		s.state[msg.id]++
		s.logAfter(msg.message)
		*resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Up, State: %d}", msg.id, s.state[msg.id])}

	case "CountDown":
		s.logBefore(msg.message)
		s.state[msg.id]--
		s.logAfter(msg.message)
		*resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("{Client: %s Counted Down, State: %d}", msg.id, s.state[msg.id])}

	case "Close":
		*resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: "Connection closed"}

	default:
		*resp = types.Response{Type: "client", Id: s.id, ReqNum: msg.message.ReqNum, Response: "Unknown request"}
	}
}

func (s *server) handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		_ = conn.Close()
	}(conn)
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

		request := internalMessage{
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
