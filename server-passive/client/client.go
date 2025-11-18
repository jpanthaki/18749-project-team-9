package main

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type client struct {
	id          string
	serverAddrs map[string]string
	connMutex   sync.Mutex
	conns       map[string]net.Conn
	primary     string

	reqNum int

	logger *log.Logger
}

func (c *client) connectToServer(id, addr string) {
	for {
		conn := c.retryDial(addr)

		c.connMutex.Lock()
		c.conns[id] = conn
		c.connMutex.Unlock()

		fmt.Printf("connected to %s\n", id)
		return
	}
}

func (c *client) retryDial(addr string) net.Conn {
	for {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			return conn
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *client) handleConnFailure(id string, conn net.Conn) {
	c.connMutex.Lock()
	conn.Close()
	delete(c.conns, id)
	go c.connectToServer(id, c.serverAddrs[id])
	c.connMutex.Unlock()
}

func (c *client) sendToAll(msg types.Message) (types.Response, error) {
	type result struct {
		resp types.Response
		err  error
	}

	responses := make(chan result, len(c.serverAddrs))

	c.connMutex.Lock()
	snapshot := make(map[string]net.Conn, len(c.conns))
	for id, conn := range c.conns {
		snapshot[id] = conn
	}
	c.connMutex.Unlock()

	var wg sync.WaitGroup

	for id, conn := range snapshot {
		wg.Add(1)
		go func(id string, conn net.Conn) {
			fmt.Println("Sending to...", id)
			enc := json.NewEncoder(conn)
			if err := enc.Encode(msg); err != nil {
				responses <- result{err: fmt.Errorf("[%s] send error: %w", id, err)}
				c.handleConnFailure(id, conn)
				return
			}
			fmt.Println("Sent to", id)
			var resp types.Response
			conn.SetReadDeadline(time.Now().Add(2000 * time.Millisecond))
			dec := json.NewDecoder(conn)
			if err := dec.Decode(&resp); err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					responses <- result{err: fmt.Errorf("[%s] recv error: %w", id, err)}
					return
				}
				responses <- result{err: fmt.Errorf("[%s] recv error: %w", id, err)}
				c.handleConnFailure(id, conn)
				return
			}

			fmt.Println("Response from", id)

			responses <- result{resp: resp, err: nil}
			wg.Done()
		}(id, conn)
	}

	if len(snapshot) == 0 {
		return types.Response{}, fmt.Errorf("no servers currently connected")
	}

	first := <-responses

	go func() {
		wg.Wait()
		close(responses)
	}()

	return first.resp, first.err
}

func main() {
	serverAddrs := map[string]string{
		"S1": "127.0.0.1:8081",
		"S2": "127.0.0.1:8082",
		"S3": "127.0.0.1:8083",
	}

	c := &client{
		id:          "C1",
		serverAddrs: serverAddrs,
		connMutex:   sync.Mutex{},
		conns:       make(map[string]net.Conn),
		primary:     "S1",
		reqNum:      0,
		logger:      log.New("Client"),
	}

	for id, addr := range c.serverAddrs {
		go c.connectToServer(id, addr)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter message type (Init, CountUp, CountDown, Close) or 'exit' to quit:")

	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = input[:len(input)-1] // Remove newline
		if input == "exit" {
			fmt.Println("Exiting client.")
			break
		}

		msg := types.Message{
			Type:    "client",
			Id:      c.id,
			ReqNum:  c.reqNum,
			Message: input,
		}

		c.reqNum++

		resp, err := c.sendToAll(msg)
		if err != nil {
			fmt.Println("Error:", err)
			continue
		}

		fmt.Println("Fastest server replied:", resp)
	}
}
