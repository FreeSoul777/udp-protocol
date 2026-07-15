package server

import (
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	cfg := Config{
		ListenAddr: "127.0.0.1:0",
	}

	srv := New(cfg)

	if srv.cfg.Workers <= 0 {
		t.Error("Expected default workers > 0")
	}
}

func TestStatsInitial(t *testing.T) {
	cfg := Config{
		ListenAddr: "127.0.0.1:9000",
	}

	srv := New(cfg)

	stats := srv.stats
	if stats.Received != 0 || stats.Valid != 0 || stats.Invalid != 0 {
		t.Error("Expected zero initial stats")
	}
}

func TestServerStartStop(t *testing.T) {
	cfg := Config{
		ListenAddr: "127.0.0.1:0",
		Workers:    2,
	}

	srv := New(cfg)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	time.Sleep(200 * time.Millisecond)

	srv.cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Server error: %v", err)
		}
		t.Log("Server stopped successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not stop in time")
	}
}
