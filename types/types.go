package types

import "encoding/json"

type Message struct {
	Type    string          `json:"type"` // "client", "lfd", "gfd", "replica", "rm"
	Id      string          `json:"id"`
	ReqNum  int             `json:"req_num"`
	Message string          `json:"message"`           // e.g., "Init", "CountUp", "CountDown", "Close"
	Payload json.RawMessage `json:"payload,omitempty"` //used to send more complex data than commands (e.g., checkpoints)
}

type Response struct {
	Type     string `json:"type"` // "client" or "lfd"
	Id       string `json:"id"`
	ReqNum   int    `json:"req_num"` //heartbeat count for lfd
	Response string `json:"response"`
}

type Checkpoint struct {
	State         map[string]int `json:"state"`
	CheckpointNum int            `json:"checkpoint_num"`
}

//RM messages
//leader promotion (if passive)
//eventually: launch replica
