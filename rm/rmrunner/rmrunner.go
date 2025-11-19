package main

import (
	"18749-team9/rm"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := flag.Int("port", 7000, "Port for RM")
	protocol := flag.String("protocol", "tcp", "Protocol for RM")
	gfdAddr := flag.String("gfdaddr", "localhost:8000", "Address to connect to GFD (host:port)")
	flag.Parse()

	r, err := rm.NewRM(*port, *protocol, *gfdAddr)
	if err != nil {
		fmt.Printf("Failed to create RM: %v\n", err)
		os.Exit(1)
	}

	err = r.Start()
	if err != nil {
		fmt.Printf("Failed to start RM: %v\n", err)
		os.Exit(1)
	}

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down RM...")
	r.Stop()
}
