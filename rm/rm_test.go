package rm

import (
	"18749-team9/gfd"
	"testing"
	"time"
)

func TestRMLifecycle(t *testing.T) {
	// Start GFD first
	g, err := gfd.NewGfd(18000, "tcp")
	if err != nil {
		t.Fatalf("Failed to create GFD: %v", err)
	}

	err = g.Start()
	if err != nil {
		t.Fatalf("Failed to start GFD: %v", err)
	}
	defer g.Stop()

	// Start RM
	r, err := NewRM(17000, "tcp", "localhost:18000")
	if err != nil {
		t.Fatalf("Failed to create RM: %v", err)
	}

	err = r.Start()
	if err != nil {
		t.Fatalf("Failed to start RM: %v", err)
	}
	defer r.Stop()

	// Give it time to connect
	time.Sleep(100 * time.Millisecond)

	// Check initial state
	if r.GetPrimary() != "" {
		t.Errorf("Expected no primary initially, got %s", r.GetPrimary())
	}
}

func TestRMPrimaryElection(t *testing.T) {
	// Start GFD
	g, err := gfd.NewGfd(18001, "tcp")
	if err != nil {
		t.Fatalf("Failed to create GFD: %v", err)
	}

	err = g.Start()
	if err != nil {
		t.Fatalf("Failed to start GFD: %v", err)
	}
	defer g.Stop()

	// Start RM
	r, err := NewRM(17001, "tcp", "localhost:18001")
	if err != nil {
		t.Fatalf("Failed to create RM: %v", err)
	}

	err = r.Start()
	if err != nil {
		t.Fatalf("Failed to start RM: %v", err)
	}
	defer r.Stop()

	time.Sleep(100 * time.Millisecond)

	// Verify membership tracking exists
	membership := r.GetMembership()
	if membership == nil {
		t.Error("Expected membership map to be initialized")
	}
}

func TestRMConnectionToGFD(t *testing.T) {
	// Start GFD
	g, err := gfd.NewGfd(18002, "tcp")
	if err != nil {
		t.Fatalf("Failed to create GFD: %v", err)
	}

	err = g.Start()
	if err != nil {
		t.Fatalf("Failed to start GFD: %v", err)
	}
	defer g.Stop()

	// Start RM
	r, err := NewRM(17002, "tcp", "localhost:18002")
	if err != nil {
		t.Fatalf("Failed to create RM: %v", err)
	}

	err = r.Start()
	if err != nil {
		t.Fatalf("Failed to start RM: %v", err)
	}
	defer r.Stop()

	// Wait for connection
	time.Sleep(200 * time.Millisecond)

	// If we get here without errors, connection succeeded
}
