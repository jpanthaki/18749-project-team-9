package types

type Message struct {
	Type    string `json:"type"` // "client" or "lfd"
	Id      string `json:"id"`
	ReqNum  int    `json:"req_num"`
	Message string `json:"message"` // e.g., "Init", "CountUp", "CountDown", "Close"
}

type Response struct {
	Type     string `json:"type"` // "client" or "lfd"
	Id       string `json:"id"`
	ReqNum   int    `json:"req_num"` //heartbeat count for lfd
	Response string `json:"response"`
}
