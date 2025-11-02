package server

import (
	"18749-team9/types"
	"fmt"
)

func (s *server) logReceived(msg types.Message) {
	logMsg := fmt.Sprintf("Received <%s, %s, %d, %s>", msg.Id, s.id, msg.ReqNum, msg.Message)
	s.logger.Log(logMsg, "MessageReceived")
}

func (s *server) logSent(resp types.Response) {
	logMsg := fmt.Sprintf("Sending <%s, %s, %d, %s>", resp.Id, s.id, resp.ReqNum, resp.Response)
	s.logger.Log(logMsg, "MessageSent")
}

func (s *server) logBefore(msg types.Message) {
	logMsg := fmt.Sprintf("State = %v before processing <%s, %s, %d, %s>", s.state, msg.Id, s.id, msg.ReqNum, msg.Message)
	s.logger.Log(logMsg, "StateBefore")
}

func (s *server) logAfter(msg types.Message) {
	logMsg := fmt.Sprintf("State = %v after processing <%s, %s, %d, %s>", s.state, msg.Id, s.id, msg.ReqNum, msg.Message)
	s.logger.Log(logMsg, "StateAfter")
}

func (s *server) logHeartbeatReceived(msg types.Message) {
	logMsg := fmt.Sprintf("<%d> Received heartbeat from %s", msg.ReqNum, msg.Id)
	s.logger.Log(logMsg, "HeartbeatReceived")
}

func (s *server) logHeartbeatSent(resp types.Response) {
	logMsg := fmt.Sprintf("<%d> Sent heartbeat to %s", resp.ReqNum, resp.Id)
	s.logger.Log(logMsg, "HeartbeatSent")
}

func (s *server) logCheckpointSent(peerId string, msg types.Message, chk types.Checkpoint) {
	logMsg := fmt.Sprintf("Checkpoint <%d> sent to %s, state: %v", msg.ReqNum, peerId, chk.State)
	s.logger.Log(logMsg, "CheckpointSent")
}

func (s *server) logCheckpointReceived(msg types.Message, chk types.Checkpoint) {
	logMsg := fmt.Sprintf("Checkpoint <%d> received from %s, state: %v", msg.ReqNum, msg.Id, chk.State)
	s.logger.Log(logMsg, "CheckpointReceived")
}
