package main

import (
	"18749-team9/gfd"
	"18749-team9/helpers"
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	port := flag.Int("port", 9090, "GFD port")
	protocol := flag.String("protocol", "tcp", "protocol")
	flag.Parse()

	fmt.Printf("Starting GFD at %s:%d, Protocol: %s\n", helpers.GetLocalIP(), *port, *protocol)

	g, err := gfd.NewGfd(*port, *protocol)
	if err != nil {
		panic(err)
	}
	err = g.Start()
	if err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		cmd := strings.TrimSpace(scanner.Text())
		if cmd == "stop" {
			err := g.Stop()
			if err != nil {
				fmt.Println("Error stopping GFD:", err)
			} else {
				fmt.Println("GFD stopped.")
			}
			break
		}
	}
}
