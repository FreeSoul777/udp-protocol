package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/telecom/udp-protocol/internal/client"
)

func main() {
	serverAddr := flag.String("server", "127.0.0.1:9000", "server address")
	packets := flag.Uint64("packets", 10000, "number of packets to send")
	workers := flag.Int("workers", 10, "number of workers (goroutines)")
	timeout := flag.Duration("timeout", 5*time.Second, "client timeout")
	flag.Parse()

	cfg := client.Config{
		ServerAddr:  *serverAddr,
		PacketCount: *packets,
		Workers:     *workers,
		Timeout:     *timeout,
	}

	c, err := client.New(cfg)
	if err != nil {
		log.Fatalf("Create client: %v", err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutdown signal received, stopping...")
		c.Stop()
	}()

	if err := c.Run(); err != nil {
		log.Fatalf("Client error: %v", err)
	}
}
