package main

import (
	"18749-team9/client"
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	s1 := flag.String("s1", "127.0.0.1:8081", "S1 address")
	s2 := flag.String("s2", "127.0.0.1:8082", "S2 address")
	s3 := flag.String("s3", "127.0.0.1:8083", "S3 address")
	id := flag.String("id", "C1", "client id")
	startReq := flag.Int("startReq", 101, "starting request number")
	timeout := flag.Duration("timeout", 3*time.Second, "per-reply timeout")
	auto := flag.Bool("auto", true, "run infinite loop automatically")
	op := flag.String("op", "CountUp", "command to send each iteration: Init|CountUp|CountDown|Close")
	think := flag.Duration("think", 0, "optional think time between requests")
	flag.Parse()

	cl, err := client.New(client.Options{
		S1Addr:      *s1,
		S2Addr:      *s2,
		S3Addr:      *s3,
		ID:          *id,
		StartingReq: *startReq,
		Timeout:     *timeout,
		Auto:        *auto,
		Op:          *op,
		Think:       *think,
	})
	if err != nil {
		fmt.Println("Client init error:", err)
		os.Exit(1)
	}
	defer cl.Close()

	fmt.Printf("Client %s connected to S1=%s S2=%s S3=%s\n", *id, *s1, *s2, *s3)

	if *auto {
		for {
			resp, err := cl.SendAll(*op)
			if err != nil {
				fmt.Println("SendAll error:", err)
			} else if resp != nil {
				fmt.Printf("First reply payload: %+v\n", *resp)
			}
			if *think > 0 {
				time.Sleep(*think)
			}
		}
	}

	// Manual mode: read commands, but still fan-out each one
	fmt.Println("Enter: Init | CountUp | CountDown | Close | exit")
	in := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !in.Scan() {
			if err := in.Err(); err != nil {
				fmt.Println("Stdin error:", err)
			}
			return
		}
		cmd := strings.TrimSpace(in.Text())
		if cmd == "" {
			continue
		}
		if strings.EqualFold(cmd, "exit") {
			fmt.Println("Exiting client.")
			return
		}
		resp, err := cl.SendAll(cmd)
		if err != nil {
			fmt.Println("SendAll error:", err)
			continue
		}
		if resp != nil {
			fmt.Printf("First reply payload: %+v\n", *resp)
		}
		if strings.EqualFold(cmd, "Close") {
			fmt.Println("Client requested close (will keep sockets up until servers close).")
		}
	}
}
