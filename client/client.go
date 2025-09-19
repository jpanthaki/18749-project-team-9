type Client interface {
	Start() error
	Stop() error
	Status() string
}

type client struct {
	id string
	message_count int
	port int
	protocol string
	status string
	conn net.Conn
	msgCh    chan ClientMessage
	closeCh  chan struct{}
	statusCh chan (chan string)

	connections map[net.Conn]struct{} // Track active connections
}

func NewClient(id string, port int, protocol string) (Lfd, error) {
	c := &client{
		id:       id,
		port:     port,
		protocol: protocol,
		status:   "stopped",
		message_count: 1,

		msgCh:       make(chan ClientMessage),
		closeCh:     make(chan struct{}),
		statusCh:    make(chan (chan string)),
		connections: make(map[net.Conn]struct{}), // Initialize client map
	}
	return c, nil
}

func (c *client) Start() error {
	fmt.Println(c.port)
	c, err := net.Dial(c.protocol, ":"+strconv.Itoa(c.port))
	if err != nil {
		return err
	}
	c.conn = c
	c.status = "running"
	fmt.Printf("Listening on %d\n", c.port)
	defer c.Close() // what is this

	//go l.manager()

	for {
		select {
		case <-l.closeCh:
			return nil
		default:
			// accept some inputs
			err := l.Heartbeat()
			if err != nil {
				l.status = "stopped"
				l.conn.Close()
				return fmt.Errorf("heartbeat failed: %w", err)
			}
			time.Sleep(time.Duration(l.heartbeat_frequency) * time.Second)
		}
	}
}

func (c *client) Stop() error {
	close(c.closeCh)
	if c.conn != nil {
		c.conn.Close()
	}
	c.status = "stopped"
	return nil
}

func (c *client) sendMessage(string message) {
	msg := types.Message{
		Type: "client",
		Id: c.id,
		ReqNum: c.message_count,
		Message: message,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	_, err = c.conn.Write(data)
	now := time.Now().Format("2006-01-02 15:04:05")
	fmt.Printf("[%s] [%d] %s sending message to %s\n", now, c.message_count, c.id, "S1")
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	// do i need to set a deadline
	buf := make([]byte, 1024)
	n, err := c.conn.Read(buf)
	if err != nil {
		return fmt.Errorf("no response from server: %w", err)
	}
	fmt.Printf("[%s] [%d] %s receives message from %s\n", now, c.message_count, c.id, "S1")
	var resp types.Response
	err = json.Unmarshal(buf[:n], &resp)
	if err != nil {
		return fmt.Errorf("invalid response from server: %w", err)
	}
	fmt.Printf("Received response: %+v\n", resp)
	c.message_count++
	return nil
}