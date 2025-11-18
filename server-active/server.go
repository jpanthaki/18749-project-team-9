package server

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

type server struct {
	id       string
	port     int
	protocol string
	listener net.Listener

	checkpointCount int
	checkpointCh    chan types.Checkpoint
	peers           map[string]string
	highWatermark   int
	lastReqNum      int

	state     map[string]int
	isReady   bool
	status    string
	msgCh     chan internalMessage
	readyCh   chan struct{}
	readyOnce sync.Once
	closeCh   chan struct{}

	connections     map[net.Conn]struct{} // Track active connections
	peerConnections map[string]net.Conn   //Track other servers
	peerMu          sync.Mutex
	connMu          sync.Mutex
	watermarkMu     sync.Mutex

	lfdPort int
	lfdConn net.Conn

	logger *log.Logger
}

type internalMessage struct {
	id         string
	message    types.Message
	responseCh chan interface{}

	conn net.Conn
}

func NewServer(id string, port int, protocol string, lfdPort int, peers map[string]string) (Server, error) {
	s := &server{
		id:       id,
		port:     port,
		protocol: protocol,
		state:    make(map[string]int),
		isReady:  false,
		status:   "stopped",

		checkpointCount: 0,
		checkpointCh:    make(chan types.Checkpoint),
		peers:           peers,
		highWatermark:   0,
		lastReqNum:      0,

		msgCh:           make(chan internalMessage),
		closeCh:         make(chan struct{}),
		readyCh:         make(chan struct{}),         // closed once when server is ready
		connections:     make(map[net.Conn]struct{}), // Initialize client map
		peerConnections: make(map[string]net.Conn),
		peerMu:          sync.Mutex{},
		connMu:          sync.Mutex{},
		watermarkMu:     sync.Mutex{},

		//		readyMu:         sync.Mutex{},

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

	go s.connectToReplicas()

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
	s.connMu.Lock()
	s.peerMu.Lock()
	for conn := range s.connections {
		_ = conn.Close()
	}
	if s.lfdConn != nil {
		_ = s.lfdConn.Close()
	}
	for _, conn := range s.peerConnections {
		_ = conn.Close()
	}
	close(s.closeCh) // Signal manager goroutine to exit
	s.peerMu.Unlock()
	s.connMu.Unlock()
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

// attempt to receive checkpoint
// if no checkpoint received after 15 attempts, declares itself ready
func (s *server) connectToReplicas() {
	for id, addr := range s.peers {
		if id == s.id {
			continue
		}
		go s.dialReplica(id, addr)
	}

	// attempt to get checkpoint from replica
	msg := types.Message{
		Type:    "replica",
		Id:      s.id,
		ReqNum:  s.checkpointCount,
		Message: "Recovery",
	}
	attempts := 0
	for attempts < 15 {
		select {
		case <-s.readyCh:
			return
		default:
		}

		s.peerMu.Lock()
		for peerId, conn := range s.peerConnections {
			go func(peerId string, conn net.Conn) {
				fmt.Printf("Sent recovery request to peer %s\n", peerId)
				json.NewEncoder(conn).Encode(msg)
			}(peerId, conn)
		}
		s.peerMu.Unlock()
		attempts++
		time.Sleep(time.Millisecond * 500)
	}
	s.readyOnce.Do(func() {
		s.isReady = true
		close(s.readyCh)
		fmt.Println("Server is ready")
	})
	fmt.Printf("No replicas detected, server is ready\n")
}

func (s *server) dialReplica(peerId, addr string) {
	for {
		conn, err := net.DialTimeout(s.protocol, addr, 500*time.Millisecond)
		if err == nil {
			s.peerMu.Lock()
			s.peerConnections[peerId] = conn
			s.peerMu.Unlock()
			return
		}
	}
}

func (s *server) manager() {
	for {
		select {
		case <-s.closeCh:
			return
		case msg := <-s.msgCh:
			var resp types.Response
			switch msg.message.Type {
			case "client":
				select {
				case <-s.readyCh:
					s.logReceived(msg.message)
					s.handleClientMessage(msg, &resp)
					s.logSent(resp)
					msg.responseCh <- resp
				default:
					s.watermarkMu.Lock()
					if msg.message.ReqNum > s.highWatermark {
						s.highWatermark = msg.message.ReqNum
					}
					fmt.Printf("new watermark: %d\n", s.highWatermark)
					s.watermarkMu.Unlock()
					msg.responseCh <- resp
				}

			case "lfd":
				s.logHeartbeatReceived(msg.message)
				s.handleLFDMessage(msg, &resp)
				s.logHeartbeatSent(resp)
				msg.responseCh <- resp
			case "replica":
				s.handleReplicaMessage(msg)
				msg.responseCh <- resp
			}

		}
	}
}

// added id argument
func (s *server) sendActiveCheckpoint(reqID string) {
	s.checkpointCount++
	chk := types.Checkpoint{
		State:         cloneState(s.state),
		LastReqNum:    s.lastReqNum,
		CheckpointNum: s.checkpointCount,
	}

	bytes, _ := json.Marshal(&chk)

	msg := types.Message{
		Type:    "replica",
		Id:      s.id,
		ReqNum:  s.checkpointCount,
		Message: "Checkpoint",
		Payload: bytes,
	}

	s.peerMu.Lock()
	for peerId, conn := range s.peerConnections {
		if peerId != reqID {
			continue
		}
		go func(peerId string, conn net.Conn) {
			if err := json.NewEncoder(conn).Encode(msg); err == nil {
				s.logCheckpointSent(peerId, msg, chk)
			} else {
				s.logger.Log(fmt.Sprintf("Error sending checkpoint %v", err), "CheckpointFailed")
				// remove old connection and redial
				s.peerMu.Lock()
				delete(s.peerConnections, peerId)
				s.peerMu.Unlock()
				go s.dialReplica(peerId, s.peers[peerId])

			}
		}(peerId, conn)

	}
	s.peerMu.Unlock()
}

func (s *server) handleClientMessage(msg internalMessage, resp *types.Response) {
	s.lastReqNum = msg.message.ReqNum
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

func (s *server) handleLFDMessage(msg internalMessage, resp *types.Response) {
	*resp = types.Response{Type: "lfd", Id: s.id, ReqNum: msg.message.ReqNum, Response: fmt.Sprintf("%d", msg.message.ReqNum)}
}

// if requested, send copy of state
// otherwise set current state as checkpoint if checkpointNum >= any value on watermark
func (s *server) handleReplicaMessage(msg internalMessage) {
	switch msg.message.Message {
	case "Recovery":
		s.sendActiveCheckpoint(msg.message.Id)
	case "Checkpoint":
		var chk types.Checkpoint
		_ = json.Unmarshal(msg.message.Payload, &chk)
		select {
		case <-s.readyCh:
			return
		default:
			s.watermarkMu.Lock()
			if chk.LastReqNum < s.highWatermark {
				return
			}
			s.watermarkMu.Unlock()
			s.readyOnce.Do(func() {
				s.isReady = true
				close(s.readyCh)
				s.state = cloneState(chk.State)
				fmt.Println("Server is ready")
			})
		}
	}
}

func (s *server) handleConnection(conn net.Conn) {
	defer func(conn net.Conn) {
		_ = conn.Close()
		s.connMu.Lock()
		delete(s.connections, conn)
		s.connMu.Unlock()
	}(conn)

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
			responseCh: make(chan interface{}),
			conn:       conn,
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

func cloneState(src map[string]int) map[string]int {
	clone := make(map[string]int, len(src))
	for k, v := range src {
		clone[k] = v
	}
	return clone
}
