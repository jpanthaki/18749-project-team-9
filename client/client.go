package client

import (
	log "18749-team9/logger"
	"18749-team9/types"
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Options struct {
	S1Addr      string
	S2Addr      string
	S3Addr      string
	ID          string
	StartingReq int
	Timeout     time.Duration
	Auto        bool   // run infinite loop automatically
	Op          string // command to send each iteration: Init | CountUp | CountDown | Close
	Think       time.Duration
}

type connState struct {
	rid   string // S1|S2|S3
	addr  string
	conn  net.Conn
	dec   *json.Decoder
	alive bool
}

type Client struct {
	id      string
	reqNum  int
	timeout time.Duration

	mu     sync.Mutex
	conns  map[string]*connState // rid -> state
	respCh chan taggedResp

	logger *log.Logger
}

type taggedResp struct {
	rid string
	r   types.Response
	err error
}

func ts() string { return time.Now().Format("2006-01-02 15:04:05.000") }

func New(opts Options) (*Client, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 3 * time.Second
	}
	c := &Client{
		id:      opts.ID,
		reqNum:  opts.StartingReq,
		timeout: opts.Timeout,
		conns:   make(map[string]*connState),
		respCh:  make(chan taggedResp, 16),
		logger:  log.New("Client"),
	}

	// Prepare connection map
	add := func(rid, addr string) {
		if strings.TrimSpace(addr) == "" {
			return
		}
		c.conns[rid] = &connState{rid: rid, addr: addr}
	}
	add("S1", opts.S1Addr)
	add("S2", opts.S2Addr)
	add("S3", opts.S3Addr)

	// Bring up connections (best-effort)
	for _, st := range c.conns {
		_ = c.dial(st) // mark alive if success
	}

	// Start reader goroutines for each alive conn
	for _, st := range c.conns {
		if st.alive {
			go c.reader(st)
		}
	}

	return c, nil
}

func (c *Client) dial(st *connState) error {
	conn, err := net.Dial("tcp", st.addr)
	if err != nil {
		st.alive = false
		return err
	}
	st.conn = conn
	st.dec = json.NewDecoder(bufio.NewReader(conn))
	st.alive = true
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, st := range c.conns {
		if st.conn != nil {
			st.conn.Close()
			st.conn = nil
			st.alive = false
		}
	}
	close(c.respCh)
	return nil
}

func (c *Client) reader(st *connState) {
	for {
		if !st.alive || st.conn == nil {
			return
		}
		_ = st.conn.SetReadDeadline(time.Now().Add(c.timeout))
		var resp types.Response
		if err := st.dec.Decode(&resp); err != nil {
			c.respCh <- taggedResp{rid: st.rid, err: err}
			// mark dead and exit; SendAll may attempt to reconnect
			c.mu.Lock()
			if st.conn != nil {
				st.conn.Close()
			}
			st.conn = nil
			st.alive = false
			c.mu.Unlock()
			return
		}
		c.respCh <- taggedResp{rid: st.rid, r: resp, err: nil}
	}
}

// SendAll sends the same command to all alive replicas, waits for the first
// reply for the current reqNum, prints duplicates/stale as they arrive, then
// increments reqNum and returns the first reply.
func (c *Client) SendAll(command string) (*types.Response, error) {
	// 1) Send to all alive
	c.mu.Lock()
	var logMsg string
	cur := c.reqNum
	for _, st := range c.conns {
		if !st.alive {
			// best-effort reconnect
			_ = c.dial(st)
			if st.alive {
				go c.reader(st)
			}
		}
		if !st.alive || st.conn == nil {
			continue
		}
		msg := types.Message{
			Type:    "client",
			Id:      c.id,
			ReqNum:  cur,
			Message: command,
		}
		data, _ := json.Marshal(msg)
		_, err := st.conn.Write(data)
		if err != nil {
			// mark dead
			st.conn.Close()
			st.conn = nil
			st.alive = false
			continue
		}
		logMsg = fmt.Sprintf("[%s] Sent <%s, %s, %d, request>\n", ts(), c.id, st.rid, cur)
		c.logger.Log(logMsg, "MessageSent")
	}
	c.mu.Unlock()

	// 2) Wait for first matching reply, log duplicates
	seen := map[string]bool{}
	var first *types.Response
	deadline := time.Now().Add(c.timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			if first == nil {
				return nil, fmt.Errorf("timeout waiting for replies for req %d", cur)
			}
			break
		}

		select {
		case tr, ok := <-c.respCh:
			var logMsg string
			if !ok {
				if first == nil {
					return nil, fmt.Errorf("client closed")
				}
				break
			}
			if tr.err != nil {
				// connection died; ignore here (we’ll keep going with others)
				continue
			}
			// Expect server to set Response.Id = replica_id (S1/S2/S3)
			rid := tr.r.Id
			rnum := tr.r.ReqNum

			switch {
			case rnum < cur:
				logMsg = fmt.Sprintf("[%s] request_num %d: Late/stale reply from %s discarded\n", ts(), rnum, rid)
				c.logger.Log(logMsg, "MessageReceived")
			case rnum > cur:
				// future reply (shouldn’t happen), ignore
			default: // rnum == cur
				if !seen[rid] {
					if first == nil {
						logMsg = fmt.Sprintf("[%s] Received <%s, %s, %d, reply>\n", ts(), c.id, rid, rnum)
						c.logger.Log(logMsg, "MessageReceived")
						cp := tr.r
						first = &cp
						seen[rid] = true
						// we can return soon; keep loop just a tad to catch very fast dups or break immediately
					} else {
						logMsg = fmt.Sprintf("[%s] request_num %d: Discarded duplicate reply from %s\n", ts(), rnum, rid)
						c.logger.Log(logMsg, "MessageReceived")
						seen[rid] = true
					}
				} else {
					logMsg = fmt.Sprintf("[%s] request_num %d: Discarded duplicate reply from %s\n", ts(), rnum, rid)
					c.logger.Log(logMsg, "MessageReceived")
				}
			}
			if first != nil {
				// we got the first — advance req and return
				c.mu.Lock()
				c.reqNum++
				c.mu.Unlock()
				return first, nil
			}
		case <-time.After(10 * time.Millisecond):
			// small tick to honor deadline
		}
	}
	// should not reach here
	return first, nil
}
