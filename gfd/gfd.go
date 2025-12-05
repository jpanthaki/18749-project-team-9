package gfd

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
)

type gfd struct {
	port     int
	protocol string
	listener net.Listener

	membership  map[string]bool
	memberCount int

	lfdConns        map[string]net.Conn // maps serverID to LFD connection
	connToServerId  map[net.Conn]string // inversion of lfdConns map
	connections     map[net.Conn]struct{}
	rmConn          net.Conn // Connection to RM
	sendPromotion   bool     // Send promotion flag
	sendPromotionId string   // Server ID to promote
	sendRelaunch    bool     // Send relaunch flag
	sendRelaunchId  string   // Server ID to relaunch

	lfdConnMu       sync.Mutex
	sendPromotionMu sync.Mutex // Protects sendPromotion access

	msgCh chan struct {
		id         string
		message    types.Message
		conn       net.Conn
		responseCh chan types.Message
	}
	closeCh chan struct{}

	logger *log.Logger
}

func NewGfd(port int, protocol string) (Gfd, error) {
	g := &gfd{
		port:            port,
		protocol:        protocol,
		membership:      make(map[string]bool),
		memberCount:     0,
		connections:     make(map[net.Conn]struct{}),
		lfdConns:        make(map[string]net.Conn),
		connToServerId:  make(map[net.Conn]string),
		lfdConnMu:       sync.Mutex{},
		sendPromotionMu: sync.Mutex{}, // Protects sendPromotion access
		msgCh: make(chan struct {
			id         string
			message    types.Message
			conn       net.Conn
			responseCh chan types.Message
		}),
		closeCh: make(chan struct{}),
		logger:  log.New("GFD"),
	}
	return g, nil
}

func (g *gfd) Start() error {
	l, err := net.Listen(g.protocol, fmt.Sprintf(":%d", g.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", g.port, err)
	}

	g.listener = l

	ready := make(chan struct{})
	go g.manager()
	go g.listen(ready)

	<-ready
	logMsg := fmt.Sprintf("GFD: %d members", g.memberCount)
	g.logger.Log(logMsg, "GFDStarted")
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
			var resp types.Message
			var logMsg string
			switch msg.message.Message {
			case "add":
				g.membership[msg.id] = true
				//g.memberCount++

				// Store the connection-to-serverID mapping AND the LFD connection immediately
				g.lfdConnMu.Lock()
				g.connToServerId[msg.conn] = msg.id
				g.lfdConns[msg.id] = msg.conn // Store connection by serverID immediately
				g.lfdConnMu.Unlock()

				livingServers := strings.Join(func(m map[string]bool) []string {
					var s []string
					for k, v := range m {
						if v {
							s = append(s, k)
						}
					}
					return s
				}(g.membership), ",")
				resp = types.Message{Type: "gfd", Id: msg.id, ReqNum: g.memberCount, Message: "Added"}
				msg.responseCh <- resp
				g.notifyRM("add", msg.id)
				g.updateMemberCount()
				logMsg := fmt.Sprintf("Adding server %s. GFD: %d members: %s", msg.id, g.memberCount, livingServers)
				g.logger.Log(logMsg, "AddingServer")
			case "remove":
				g.membership[msg.id] = false
				//g.memberCount--
				livingServers := strings.Join(func(m map[string]bool) []string {
					var s []string
					for k, v := range m {
						if v {
							s = append(s, k)
						}
					}
					return s
				}(g.membership), ",")
				resp = types.Message{Type: "gfd", Id: msg.id, ReqNum: g.memberCount, Message: "Removed"}
				msg.responseCh <- resp

				// Notify RM of membership change
				g.notifyRM("remove", msg.id)
				g.updateMemberCount()
				logMsg := fmt.Sprintf("Removing server %s. GFD: %d members: %s", msg.id, g.memberCount, livingServers)
				g.logger.Log(logMsg, "RemovingServer")
			case "heartbeat":
				g.lfdConnMu.Lock()
				if _, ok := g.lfdConns[msg.id]; !ok {
					g.lfdConns[msg.id] = msg.conn
				}
				g.lfdConnMu.Unlock()
				logMsg = fmt.Sprintf("[%d] GFD receives heartbeat from %s", msg.message.ReqNum, msg.id)
				g.logger.Log(logMsg, "LFDHeartbeatReceived")
				g.sendPromotionMu.Lock()
				var message string
				message = "Alive"
				if g.sendPromotion && g.sendPromotionId[len(g.sendPromotionId)-1] == msg.id[len(msg.id)-1] {
					message = "Promote"
					g.sendPromotion = false
					g.sendPromotionId = ""
					g.logger.Log(fmt.Sprintf("GFD sending promotion to %s", msg.id), "PromotionSent")
				} else if g.sendRelaunch && g.sendRelaunchId[len(g.sendRelaunchId)-1] == msg.id[len(msg.id)-1] {
					message = "Relaunch"
					g.sendRelaunchId = ""
					g.sendRelaunch = false
					g.logger.Log(fmt.Sprintf("GFD sending relaunch to %s", msg.id), "RelaunchSent")
				}
				g.sendPromotionMu.Unlock()
				var responseMessage types.Message
				responseMessage = types.Message{Type: "gfd", Id: msg.id, ReqNum: msg.message.ReqNum, Message: message}
				msg.responseCh <- responseMessage
				logMsg = fmt.Sprintf("[%d] GFD sending heartbeat ACK to %s", msg.message.ReqNum, msg.id)
				g.logger.Log(logMsg, "LFDHeartbeatSent")
			}
		case <-g.closeCh:
			return
		}
	}
}

func (g *gfd) notifyRM(action string, serverId string) {
	if g.rmConn == nil {
		return
	}

	msg := types.Message{
		Type:    "gfd",
		Id:      serverId,
		Message: action,
		ReqNum:  0,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// Check again before write to avoid race condition
	if g.rmConn == nil {
		return
	}

	_, err = g.rmConn.Write(msgBytes)
	if err != nil {
		// RM connection lost
		g.rmConn = nil
	}
}

func (g *gfd) forwardPromotionToLFD(serverID string) {
	g.lfdConnMu.Lock()
	conn, exists := g.lfdConns[serverID]
	g.logger.Log(fmt.Sprintf("Forwarding promotion to LFD for server %s and conn %v", serverID, conn), "PromotionForwardAttempt")
	g.lfdConnMu.Unlock()

	if !exists {
		logMsg := fmt.Sprintf("Cannot forward promotion: no LFD connection for server %s", serverID)
		g.logger.Log(logMsg, "PromotionForwardFailed")
		return
	}

	logMsg2 := fmt.Sprintf("Forwarding promotion to LFD for server %s", serverID)
	g.logger.Log(logMsg2, "PromotionForward")

	msg := types.Message{
		Type:    "rm", // Changed from "gfd" to "rm" for consistency
		Id:      serverID,
		Message: "Promote", // Changed from "promote" to "Promote" to match handleRMMessage
		ReqNum:  0,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_, err = conn.Write(msgBytes)
	if err != nil {
		g.lfdConnMu.Lock()
		delete(g.lfdConns, serverID)
		g.lfdConnMu.Unlock()
		return
	}

	logMsg := fmt.Sprintf("Forwarding promotion to LFD for server %s", serverID)
	g.logger.Log(logMsg, "PromotionForwarded")
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

	// Read first message to determine connection type
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

	// Check if this is RM registering
	if msg.Type == "rm" && msg.Message == "register" {
		g.rmConn = conn
		logMsg := "RM registered with GFD"
		g.logger.Log(logMsg, "RMRegistered")

		// Send initial membership to RM
		initialMsg := types.Message{
			Type:    "gfd",
			Id:      "",
			Message: "initial",
			ReqNum:  0,
		}
		initialBytes, _ := json.Marshal(initialMsg)
		conn.Write(initialBytes)

		dec := json.NewDecoder(conn)

		// Keep connection open for RM and listen for promotion messages
		for {

			// Parse message from RM
			var rmMsg types.Message
			if err := dec.Decode(&rmMsg); err != nil {
				logMsg = fmt.Sprintf("Failed to parse RM message: %v", err)
				g.logger.Log(logMsg, "RMMessageParseFailed")
				continue
			}

			logMsg = fmt.Sprintf("Parsed RM message: %v", rmMsg)
			g.logger.Log(logMsg, "RMMessageParsed")

			// Handle promotion message
			if rmMsg.Message == "Promote" {
				g.sendPromotionMu.Lock()
				g.sendPromotion = true
				g.sendPromotionId = rmMsg.Id
				g.sendPromotionMu.Unlock()
				logMsg := fmt.Sprintf("Scheduled promotion for server %s", rmMsg.Id)
				g.logger.Log(logMsg, "PromotionScheduled")
				continue
			}

			if rmMsg.Message == "relaunch" {
				g.sendPromotionMu.Lock()
				g.sendRelaunch = true
				g.sendRelaunchId = rmMsg.Id
				logMsg := fmt.Sprintf("Scheduled relaunch for server %s", rmMsg.Id)
				g.logger.Log(logMsg, "RelaunchScheduled")
				g.sendPromotionMu.Unlock()
			}
		}
	}

	// Handle LFD connections
	if msg.Type != "lfd" {
		return
	}

	// Process first LFD message
	request := struct {
		id         string
		message    types.Message
		conn       net.Conn
		responseCh chan types.Message
	}{
		id:         msg.Id,
		message:    msg,
		conn:       conn,
		responseCh: make(chan types.Message),
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

	// Continue handling subsequent LFD messages
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
			conn       net.Conn
			responseCh chan types.Message
		}{
			id:         msg.Id,
			message:    msg,
			conn:       conn,
			responseCh: make(chan types.Message),
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

func (g *gfd) updateMemberCount() {
	memberCount := 0
	for _, alive := range g.membership {
		if alive {
			memberCount++
		}
	}
	g.memberCount = memberCount
}
