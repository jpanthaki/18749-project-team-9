package registry

import (
	"18749-team9/helpers"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type message struct {
	header string
	body   map[string]string
}

type registry struct {
	port  int
	table map[string]map[string]string

	conn *net.UDPConn

	closeCh chan struct{}

	token string
}

func NewRegistry(port int, token string) (Registry, error) {
	reg := &registry{
		port:  port,
		table: make(map[string]map[string]string),
		token: token,
	}

	return reg, nil
}

func (r *registry) Start() error {
	go r.manager()
	return nil
}

func (r *registry) Stop() error {
	close(r.closeCh)
	return nil
}

func (r *registry) manager() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return
	}

	r.conn = conn
	defer func(conn *net.UDPConn) {
		err := conn.Close()
		if err != nil {
			return
		}
	}(r.conn)

	buffer := make([]byte, 1024)
	for {
		select {
		case <-r.closeCh:
			return
		default:
			err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				return
			}

			_, rAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				var netErr net.Error
				if errors.As(err, &netErr) && netErr.Timeout() {
					continue
				}
				continue
			}

			var msg message

			err = json.Unmarshal(buffer, &msg)
			if err != nil {
				continue
			}

			if msg.body["token"] != r.token {
				continue
			}

			var resp message
			switch msg.header {
			case "discover":
				resp = message{
					header: "response",
					body: map[string]string{
						"address": fmt.Sprintf("%s:%d", helpers.GetLocalIP(), r.port),
						"token":   r.token,
					},
				}
			case "register":
				role := msg.body["role"]
				id := msg.body["id"]
				nodeAddr := msg.body["addr"]
				r.table[role][id] = nodeAddr

				resp = message{
					header: "response",
					body: map[string]string{
						"registered": "true",
					},
				}
			case "lookup":
				role := msg.body["role"]
				id := msg.body["id"]

				if nodeAddr, ok := r.table[role][id]; !ok {
					resp = message{
						header: "response",
						body: map[string]string{
							"found": "false",
							"addr":  "",
						},
					}
				} else {
					resp = message{
						header: "response",
						body: map[string]string{
							"found": "true",
							"addr":  nodeAddr,
						},
					}
				}
			}

			respBytes, err := json.Marshal(resp)

			_, err = r.conn.WriteToUDP(respBytes, rAddr)
			if err != nil {
				continue
			}
		}
	}
}
