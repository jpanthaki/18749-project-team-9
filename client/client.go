package client

import (
	"18749-team9/types"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type Options struct {
	Addr        string
	ID          string
	ServerID    string
	StartingReq int
	Timeout     time.Duration
}

type Client struct {
	id       string
	serverID string
	reqNum   int
	conn     net.Conn
	dec      *json.Decoder
	timeout  time.Duration
}

func ts() string { return time.Now().Format("2006-01-02 15:04:05") }

func New(opts Options) (*Client, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 3 * time.Second
	}
	conn, err := net.Dial("tcp", opts.Addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", opts.Addr, err)
	}
	return &Client{
		id:       opts.ID,
		serverID: opts.ServerID,
		reqNum:   opts.StartingReq,
		conn:     conn,
		dec:      json.NewDecoder(conn),
		timeout:  opts.Timeout,
	}, nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Client) Send(command string) (*types.Response, error) {
	// Format from the rubric
	fmt.Printf("[%s] Sent <%s, %s, %d, request>\n", ts(), c.id, c.serverID, c.reqNum)

	msg := types.Message{
		Type:    "client",
		Id:      c.id,
		ReqNum:  c.reqNum,
		Message: command,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	if _, err := c.conn.Write(data); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
	defer c.conn.SetReadDeadline(time.Time{})

	var resp types.Response
	if err := c.dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	fmt.Printf("[%s] Received <%s, %s, %d, reply>\n", ts(), c.id, c.serverID, resp.ReqNum)

	c.reqNum++

	return &resp, nil
}
