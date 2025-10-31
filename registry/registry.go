package registry

import (
	"errors"
	"fmt"
)

func NewRegistry() (Registry, error) {
	reg := &registry{
		table:      make(map[string]map[string]string),
		closeCh:    make(chan struct{}),
		lookupCh:   make(chan lookupRequest),
		registerCh: make(chan registerRequest),
	}

	return reg, nil
}

func (r *registry) Start() error {
	go r.manager()
	return nil
}

func (r *registry) Stop() error {
	close(r.closeCh)
	return nil
}

func (r *registry) Lookup(role string, id string) (string, error) {
	req := lookupRequest{
		role:       role,
		id:         id,
		responseCh: make(chan string),
	}
	r.lookupCh <- req
	response := <-req.responseCh

	if response == "ROLE_NOT_FOUND" {
		return "", errors.New(fmt.Sprintf("Role %s not found", role))
	}

	if response == "ID_NOT_FOUND" {
		return "", errors.New(fmt.Sprintf("ID %s not found for role %s.", id, role))
	}

	return response, nil
}

func (r *registry) Register(role string, id string, addr string) {
	req := registerRequest{
		role: role,
		id:   id,
		addr: addr,
	}
	r.registerCh <- req
}

type lookupRequest struct {
	role       string
	id         string
	responseCh chan string
}

type registerRequest struct {
	role string
	id   string
	addr string
}

type registry struct {
	table      map[string]map[string]string
	closeCh    chan struct{}
	lookupCh   chan lookupRequest
	registerCh chan registerRequest
}

func (r *registry) manager() {
	for {
		select {
		case <-r.closeCh:
			return
		case req := <-r.lookupCh:
			role, id := req.role, req.id
			if inner, ok := r.table[role]; !ok {
				req.responseCh <- "ROLE_NOT_FOUND"
			} else if addr, ok := inner[id]; !ok {
				req.responseCh <- "ID_NOT_FOUND"
			} else {
				req.responseCh <- addr
			}
		case req := <-r.registerCh:
			role, id, addr := req.role, req.id, req.addr
			if _, ok := r.table[role]; !ok {
				r.table[role] = make(map[string]string)
			}
			r.table[role][id] = addr
		}
	}
}
