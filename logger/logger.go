package logger

import (
	"log"
	"os"
	"time"
)

type Logger struct {
	Component string
	Base      *log.Logger
}

func New(component string) *Logger {
	return &Logger{
		Component: component,
		Base:      log.New(os.Stdout, "", 0),
	}
}

func (l *Logger) Log(msg string, msgType string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	color, ok := componentColors[l.Component][msgType]
	if !ok {
		color = White
	}
	l.Base.Printf("%s[%s] %s%s\n", color, now, msg, Reset)
}
