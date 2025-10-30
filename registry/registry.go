package registry

import (
	"18749-team9/helpers"
	"encoding/json"
	"errors"
	"fmt"
	"net"
)

type message struct {
	Header string            `json:"header"`
	Body   map[string]string `json:"body"`
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
		port:    port,
		table:   make(map[string]map[string]string),
		token:   token,
		closeCh: make(chan struct{}),
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
	addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", r.port))
	if err != nil {
		return
	}

	fmt.Println(addr)

	conn, err := net.ListenUDP("udp4", addr)
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
			fmt.Println("here")
			//err := conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			//if err != nil {
			//	fmt.Println("Error setting read deadline:", err)
			//	return
			//}

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

			fmt.Printf("received message: %v\n", msg)

			if msg.Body["token"] != r.token {
				continue
			}

			var resp message
			switch msg.Header {
			case "discover":
				resp = message{
					Header: "response",
					Body: map[string]string{
						"address": fmt.Sprintf("%s:%d", helpers.GetLocalIP(), r.port),
						"token":   r.token,
					},
				}
			case "register":
				role := msg.Body["role"]
				id := msg.Body["id"]
				nodeAddr := msg.Body["addr"]
				r.table[role][id] = nodeAddr

				resp = message{
					Header: "response",
					Body: map[string]string{
						"registered": "true",
					},
				}
			case "lookup":
				role := msg.Body["role"]
				id := msg.Body["id"]

				if nodeAddr, ok := r.table[role][id]; !ok {
					resp = message{
						Header: "response",
						Body: map[string]string{
							"found": "false",
							"addr":  "",
						},
					}
				} else {
					resp = message{
						Header: "response",
						Body: map[string]string{
							"found": "true",
							"addr":  nodeAddr,
						},
					}
				}
			}

			respBytes, err := json.Marshal(resp)

			fmt.Printf("Sending message: %v\n", resp)

			_, err = r.conn.WriteToUDP(respBytes, rAddr)
			if err != nil {
				continue
			}
		}
	}
}
