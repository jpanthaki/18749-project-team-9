package logger

import "fmt"

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"
)

var componentColors = map[string]map[string]string{
	"Server": {
		"HeartbeatReceived":  RGB(255, 120, 255),
		"HeartbeatSent":      RGB(200, 60, 200),
		"MessageReceived":    RGB(90, 210, 255),
		"MessageSent":        RGB(40, 160, 220),
		"StateBefore":        RGB(160, 255, 160),
		"StateAfter":         RGB(60, 200, 60),
		"CheckpointSent":     RGB(255, 210, 80),
		"CheckpointReceived": RGB(255, 255, 160),
		"CheckpointFailed":   RGB(255, 100, 60),
		"LeaderPromoted":     RGB(255, 255, 0),
	},
	"LFD": {
		"GFDHeartbeatReceived":    RGB(255, 120, 255),
		"GFDHeartbeatSent":        RGB(200, 60, 200),
		"GFDMessageSent":          RGB(80, 255, 200),
		"ServerHeartbeatReceived": RGB(90, 210, 255),
		"ServerHeartbeatSent":     RGB(40, 160, 220),
		"StatusChange":            RGB(255, 80, 80),
	},
	"GFD": {
		"GFDStarted":           RGB(255, 255, 0),
		"LFDHeartbeatReceived": RGB(255, 120, 255),
		"LFDHeartbeatSent":     RGB(200, 60, 200),
		"LFDMessageReceived":   RGB(80, 255, 200),
		"AddingServer":         RGB(255, 255, 0),
		"RemovingServer":       RGB(255, 80, 80),
	},
	"Client": {
		"MessageSent":     RGB(40, 160, 220),
		"MessageReceived": RGB(90, 210, 255),
	},
}

func ComponentColors() map[string]map[string]string {
	return componentColors
}

func RGB(r, g, b int) string {
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return v
	}
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", clamp(r), clamp(g), clamp(b))
}
