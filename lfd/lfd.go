type Lfd interface {
	Start() error
	Stop() error
	Status() string
}

type lfd struct {
	id string
	heartbeat_count int
	heartbeat_frequency int
	port int
	protocol string
	status string
	conn net.Conn
	msgCh    chan ClientMessage
	closeCh  chan struct{}
	statusCh chan (chan string)

	connections map[net.Conn]struct{} // Track active connections
}

func NewLfd(heartbeat_frequency int, id string, port int, protocol string) (Lfd, error) {
	l := &lfd{
		id:       id,
		port:     port,
		protocol: protocol,
		status:   "stopped",
		heartbeat_count: 1,
		heartbeat_frequency: heartbeat_frequency,

		msgCh:       make(chan ClientMessage),
		closeCh:     make(chan struct{}),
		statusCh:    make(chan (chan string)),
		connections: make(map[net.Conn]struct{}), // Initialize client map
	}
	return l, nil
}

func (l *lfd) Start() error {
	fmt.Println(l.port)
	l, err := net.Dial(l.protocol, ":"+strconv.Itoa(l.port))
	if err != nil {
		return err
	}
	l.conn = l
	l.status = "running"
	l.isReady = true
	fmt.Printf("Listening on %d\n", l.port)
	defer l.Close()

	//go l.manager()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		fmt.Println("connected to a client")

		l.connections[conn] = struct{}{} // Add the new connection to the clients map

		go l.handleConnection(conn)
	}
}

func (l *lfd) handleSocket() {
	defer conn.Close()
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
		fmt.Println("Received message:", msg)
		// do stuff
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