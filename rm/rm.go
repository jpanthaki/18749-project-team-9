package rm

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

type RM interface {
	Start() error
	Stop() error
	GetMembership() map[string]bool
	GetPrimary() string
}

type rm struct {
	port     int
	protocol string
	gfdConn  net.Conn
	gfdPort  int

	membership  map[string]bool
	memberCount int
	primary     string // for passive replication

	msgCh   chan types.Message
	closeCh chan struct{}

	logger *log.Logger
}

func NewRM(port int, protocol string, gfdPort int) (RM, error) {
	r := &rm{
		port:        port,
		protocol:    protocol,
		gfdPort:     gfdPort,
		membership:  make(map[string]bool),
		memberCount: 0,
		msgCh:       make(chan types.Message),
		closeCh:     make(chan struct{}),
		logger:      log.New("RM"),
	}
	return r, nil
}

func (r *rm) Start() error {
	// Connect to GFD
	conn, err := net.Dial(r.protocol, fmt.Sprintf(":%d", r.gfdPort))
	if err != nil {
		return fmt.Errorf("failed to connect to GFD on port %d: %w", r.gfdPort, err)
	}
	r.gfdConn = conn

	// Send registration message
	regMsg := types.Message{Type: "rm", Id: "1", Message: "register", ReqNum: 0}
	msgBytes, _ := json.Marshal(regMsg)
	_, err = r.gfdConn.Write(msgBytes)
	if err != nil {
		return fmt.Errorf("failed to register with GFD: %w", err)
	}

	go r.manager()
	go r.listenToGFD()

	logMsg := fmt.Sprintf("RM started on port %d, connected to GFD:%d", r.port, r.gfdPort)
	r.logger.Log(logMsg, "RMStarted")
	return nil
}

func (r *rm) Stop() error {
	if r.gfdConn != nil {
		r.gfdConn.Close()
	}
	close(r.closeCh)
	return nil
}

func (r *rm) GetMembership() map[string]bool {
	return r.membership
}

func (r *rm) GetPrimary() string {
	return r.primary
}

func (r *rm) manager() {
	for {
		select {
		case msg := <-r.msgCh:
			switch msg.Message {
			case "add":
				r.membership[msg.Id] = true
				r.memberCount++
				// Only elect primary if we don't have one
				if r.primary == "" {
					r.electPrimary()
				}
				r.printMembership("Added", msg.Id)
			case "remove":
				r.membership[msg.Id] = false
				r.memberCount--
				// Only re-elect if the primary died
				if r.primary == msg.Id {
					r.electPrimary()
				}
				r.printMembership("Removed", msg.Id)
			}
		case <-r.closeCh:
			return
		}
	}
}

func (r *rm) listenToGFD() {
	defer r.gfdConn.Close()
	for {
		buf := make([]byte, 1024)
		n, err := r.gfdConn.Read(buf)
		if err != nil {
			return
		}

		var msg types.Message
		err = json.Unmarshal(buf[:n], &msg)
		if err != nil {
			continue
		}

		r.msgCh <- msg
	}
}

func (r *rm) electPrimary() {
	// Pick any living member as primary
	r.primary = ""
	for id, alive := range r.membership {
		if alive {
			r.primary = id
			logMsg := fmt.Sprintf("New primary elected: %s", r.primary)
			r.logger.Log(logMsg, "PrimaryElected")
			return
		}
	}
	// No living members
	logMsg := "No primary available (no living members)"
	r.logger.Log(logMsg, "NoPrimary")
}

func (r *rm) printMembership(action string, serverId string) {
	livingServers := strings.Join(func(m map[string]bool) []string {
		var s []string
		for k, v := range m {
			if v {
				s = append(s, k)
			}
		}
		return s
	}(r.membership), ",")

	logMsg := fmt.Sprintf("%s server %s. RM: %d members: %s. Primary: %s",
		action, serverId, r.memberCount, livingServers, r.primary)
	r.logger.Log(logMsg, "MembershipUpdate")
}
