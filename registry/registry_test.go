package registry

import (
	"testing"
	"time"
)

func TestRegisterAndLookup_Success(t *testing.T) {
	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error: %v", err)
	}
	if err := reg.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer reg.Stop()

	role, id, addr := "server", "s1", "localhost:9000"

	reg.Register(role, id, addr)

	got, err := reg.Lookup(role, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != addr {
		t.Fatalf("expected %q, got %q", addr, got)
	}
}

func TestLookup_RoleNotFound(t *testing.T) {
	reg, _ := NewRegistry()
	reg.Start()
	defer reg.Stop()

	_, err := reg.Lookup("unknownRole", "id1")
	if err == nil {
		t.Fatalf("expected error for missing role, got nil")
	}
	want := "Role unknownRole not found"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}

func TestLookup_IDNotFound(t *testing.T) {
	reg, _ := NewRegistry()
	reg.Start()
	defer reg.Stop()

	reg.Register("client", "c1", "127.0.0.1:7000")

	_, err := reg.Lookup("client", "c2")
	if err == nil {
		t.Fatalf("expected error for missing id, got nil")
	}
	want := "ID c2 not found for role client."
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
}

func TestRegister_OverwriteAllowed(t *testing.T) {
	reg, _ := NewRegistry()
	reg.Start()
	defer reg.Stop()

	role, id := "worker", "w1"
	addr1 := "127.0.0.1:1111"
	addr2 := "127.0.0.1:2222"

	reg.Register(role, id, addr1)
	reg.Register(role, id, addr2)

	got, err := reg.Lookup(role, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != addr2 {
		t.Fatalf("expected overwritten addr %q, got %q", addr2, got)
	}
}

func TestStop_CleanExit(t *testing.T) {
	reg, _ := NewRegistry()
	reg.Start()

	done := make(chan struct{})
	go func() {
		reg.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatalf("Stop() did not complete in time")
	}
}
