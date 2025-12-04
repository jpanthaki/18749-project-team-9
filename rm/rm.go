package rm

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
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
	gfdAddr  string // Changed from gfdPort to gfdAddr to support full address

	membership  map[string]bool
	memberCount int
	primary     string // for passive replication

	msgCh   chan types.Message
	closeCh chan struct{}

	logger *log.Logger
}

// Basic initialization
func NewRM(port int, protocol string, gfdAddr string) (RM, error) {
	r := &rm{
		port:        port,
		protocol:    protocol,
		gfdAddr:     gfdAddr,
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
	for {
		conn, err := net.Dial(r.protocol, r.gfdAddr)
		if err == nil {
			fmt.Printf("Connected to GFD at %s\n", r.gfdAddr)
			r.gfdConn = conn
			break
		}
		time.Sleep(1 * time.Second)
	}

	r.primary = ""

	// Send message to register with GFD
	regMsg := types.Message{Type: "rm", Id: "1", Message: "register", ReqNum: 0}
	msgBytes, _ := json.Marshal(regMsg)
	_, err := r.gfdConn.Write(msgBytes)
	if err != nil {
		return fmt.Errorf("failed to register with GFD: %w", err)
	}

	// Manage replicas and listen for GFD for failures
	go r.manager()
	go r.listenToGFD()

	logMsg := fmt.Sprintf("RM started on port %d, connected to GFD:%s", r.port, r.gfdAddr)
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
				// elect primary if this is the first member
				r.logger.Log((fmt.Sprintf("current primary: %s", r.primary)), "CurrentPrimary")
				if r.primary == "" {
					r.logger.Log(("Found empty primary, electing new primary"), "CurrentPrimary")
					r.electPrimary()
				}
				r.resetMemberCount()
				r.printMembership("Added", msg.Id)
			case "remove":
				r.membership[msg.Id] = false
				// Only re-elect if the primary died
				r.logger.Log((fmt.Sprintf("a server %s died, current primary: %s", msg.Id, r.primary)), "CurrentPrimary")
				if r.primary == msg.Id {
					r.logger.Log((fmt.Sprintf("Primary %s died, electing new primary", r.primary)), "CurrentPrimary")
					r.electPrimary()
				}
				r.resetMemberCount()
				r.printMembership("Removed", msg.Id)
				r.sendRelaunchMessage(msg.Id)
			}
		case <-r.closeCh:
			return
		}
	}
}

func (r *rm) sendRelaunchMessage(id string) {
	msg := types.Message{
		Type:    "rm",
		Id:      id,
		Message: "relaunch",
		ReqNum:  0,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_, err = r.gfdConn.Write(msgBytes)
	if err != nil {
		// GFD connection los
		r.gfdConn = nil
		return
	}

	logMsg := fmt.Sprintf("Notified GFD to relaunch replica: %s", id)
	r.logger.Log(logMsg, "RelaunchNotification")
}

func (r *rm) resetMemberCount() {
	count := 0
	for _, alive := range r.membership {
		if alive {
			count++
		}
	}
	r.memberCount = count
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

		// Feed into channel for manager
		r.msgCh <- msg
	}
}

func (r *rm) electPrimary() {
	oldPrimary := r.primary
	r.primary = ""
	for id, alive := range r.membership {
		if alive {
			r.primary = id
			logMsg := fmt.Sprintf("New primary elected: %s", r.primary)
			r.logger.Log(logMsg, "PrimaryElected")

			// Notify GFD of new primary promotion (only if primary changed)
			if oldPrimary != r.primary {
				r.notifyGFDPromotion(r.primary)
			}
			return
		}
	}
	// No living members
	logMsg := "No primary available (no living members)"
	r.logger.Log(logMsg, "NoPrimary")
}

func (r *rm) notifyGFDPromotion(primaryId string) {
	if r.gfdConn == nil {
		return
	}

	msg := types.Message{
		Type:    "rm",
		Id:      primaryId,
		Message: "Promote",
		ReqNum:  0,
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_, err = r.gfdConn.Write(msgBytes)
	if err != nil {
		// GFD connection lost
		r.gfdConn = nil
		return
	}

	logMsg := fmt.Sprintf("Notified GFD of primary promotion: %s", primaryId)
	r.logger.Log(logMsg, "PromotionNotification")
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
