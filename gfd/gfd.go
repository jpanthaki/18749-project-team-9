package gfd

import (
	"net"
)

type gfd struct {
	//TODO implement me
}

func (g gfd) Start() error {
	//TODO implement me
	panic("implement me")
}

func (g gfd) Stop() error {
	//TODO implement me
	panic("implement me")
}

func manager() {
	//TODO implement me
}

func listen() {
	//TODO implement me
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	//TODO implement me
}
