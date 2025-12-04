package main

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"bufio"
	"encoding/json"
	"flag"
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

	reqNum int

	logger *log.Logger
}

func (c *client) connectToServer(id, addr string) {
	for {
		conn := c.retryDial(addr)

		c.connMutex.Lock()
		c.conns[id] = conn
		c.connMutex.Unlock()

		// fmt.Printf("connected to %s\n", id)
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
			defer wg.Done()
			var logMsg string
			enc := json.NewEncoder(conn)
			if err := enc.Encode(msg); err != nil {
				responses <- result{err: fmt.Errorf("[%s] send error: %w", id, err)}
				c.handleConnFailure(id, conn)
				return
			}
			logMsg = fmt.Sprintf("Sent <%s, %s, %d, %s request>", c.id, id, c.reqNum, msg.Message)
			c.logger.Log(logMsg, "MessageSent")
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

			responses <- result{resp: resp, err: nil}
			// wg.Done()
		}(id, conn)
	}

	if len(snapshot) == 0 {
		return types.Response{}, fmt.Errorf("no servers currently connected")
	}

	first := <-responses

	if first.err == nil {
		logMsg := fmt.Sprintf("Received <%s, %s, %d, %s>", c.id, first.resp.Id, first.resp.ReqNum, first.resp.Response)
		c.logger.Log(logMsg, "MessageReceived")
	}

	go func() {
		wg.Wait()
		close(responses)
	}()

	for r := range responses {
		if r.err == nil && r.resp.Id != first.resp.Id {
			logMsg := fmt.Sprintf("Ignored Duplicate Response from %s", r.resp.Id)
			c.logger.Log(logMsg, "MessageIgnored")
		}
	}

	return first.resp, first.err
}

func main() {
	id := flag.String("id", "C1", "client id")
	// s1Addr := flag.String("s1", "127.0.0.1:8081", "s1 addr")
	// s2Addr := flag.String("s2", "127.0.0.1:8082", "s2 addr")
	// s3Addr := flag.String("s3", "127.0.0.1:8083", "s3 addr")
	auto := flag.Bool("auto", false, "send messages automatically")
	flag.Parse()

	// serverAddrs := map[string]string{
	// 	"S1": *s1Addr,
	// 	"S2": *s2Addr,
	// 	"S3": *s3Addr,
	// }

	cwd, _ := os.Getwd()

	data, _ := os.ReadFile(fmt.Sprintf("%s/serverAddrs.json", cwd))

	serverAddrs := make(map[string]string)

	json.Unmarshal(data, &serverAddrs)

	fmt.Println(serverAddrs)

	c := &client{
		id:          *id,
		serverAddrs: serverAddrs,
		connMutex:   sync.Mutex{},
		conns:       make(map[string]net.Conn),
		reqNum:      0,
		logger:      log.New("Client"),
	}

	var wg sync.WaitGroup

	for id, addr := range c.serverAddrs {
		wg.Add(1)
		go func(id, addr string) {
			defer wg.Done()
			c.connectToServer(id, addr)
		}(id, addr)
	}

	wg.Wait()

	if *auto {
		ticker := time.NewTicker(2000 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				msg := types.Message{
					Type:    "client",
					Id:      c.id,
					ReqNum:  c.reqNum,
					Message: "CountUp",
				}

				c.reqNum++

				_, err := c.sendToAll(msg)
				if err != nil {
					c.reqNum--
					continue
				}

			}
		}
	} else {
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
}
