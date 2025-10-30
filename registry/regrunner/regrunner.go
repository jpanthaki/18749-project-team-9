package main

import (
	"18749-team9/registry"
	"bufio"
	"fmt"
	"os"
	"strings"
)

func main() {
	reg, _ := registry.NewRegistry(18749, "team9")

	reg.Start()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		cmd := strings.TrimSpace(scanner.Text())
		if cmd == "stop" {
			err := reg.Stop()
			if err != nil {
				fmt.Println("Error stopping registry:", err)
			} else {
				fmt.Println("registry stopped.")
			}
			break
		} else {
			fmt.Println("Unknown command. Type 'stop' to stop the registry.")
		}
	}
}
