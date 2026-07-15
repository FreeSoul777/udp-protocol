package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/telecom/udp-protocol/internal/server"
)

func main() {
	listenAddr := flag.String("addr", "127.0.0.1:9000", "listen address")
	workers := flag.Int("workers", 0, "number of workers (0 = CPU count)")
	flag.Parse()

	srv := server.New(server.Config{
		ListenAddr: *listenAddr,
		Workers:    *workers,
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Shutdown signal received")
		srv.Stop()
	}()

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
