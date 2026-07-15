package client

import (
	"log"
	"sync"
	"time"

	"github.com/telecom/udp-protocol/internal/protocol"
)

const (
	// BaseRTO is the base retransmission timeout.
	BaseRTO = 20 * time.Millisecond

	// MaxRetries is the maximum number of retransmission attempts.
	MaxRetries = 5

	// RetransmitInterval is the interval between pending packet checks.
	RetransmitInterval = 100 * time.Millisecond
)

// PendingEntry holds information about an unacknowledged packet.
type PendingEntry struct {
	Packet    *protocol.Packet
	SentAt    time.Time
	Retries   int
	NextRetry time.Time
}

// PendingTracker tracks unacknowledged packets.
type PendingTracker struct {
	mu      sync.Mutex
	pending map[uint64]*PendingEntry
	acked   map[uint64]bool
}

// NewPendingTracker creates a new PendingTracker.
func NewPendingTracker() *PendingTracker {
	return &PendingTracker{
		pending: make(map[uint64]*PendingEntry),
		acked:   make(map[uint64]bool),
	}
}

func (pt *PendingTracker) IsEmpty() bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return len(pt.pending) == 0
}

// Add adds a packet to the pending set.
func (pt *PendingTracker) Add(seq uint64, p *protocol.Packet) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	pt.pending[seq] = &PendingEntry{
		Packet:    p,
		SentAt:    now,
		Retries:   0,
		NextRetry: now.Add(BaseRTO),
	}
}

// MarkAcked marks a packet as acknowledged.
func (pt *PendingTracker) MarkAcked(seq uint64) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	delete(pt.pending, seq)
	pt.acked[seq] = true
}

// GetExpired returns packets with expired retransmission timeout.
func (pt *PendingTracker) GetExpired() []*PendingEntry {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	now := time.Now()
	var expired []*PendingEntry

	for seq, entry := range pt.pending {
		if now.After(entry.NextRetry) && entry.Retries < MaxRetries {
			entry.Retries++
			backoff := BaseRTO * (1 << entry.Retries)
			entry.NextRetry = now.Add(backoff)
			entry.SentAt = now
			expired = append(expired, entry)
			log.Printf("Retransmitting Seq=%d (attempt %d/%d, backoff=%v)",
				seq, entry.Retries, MaxRetries, backoff)
		}
	}

	return expired
}

// GetLost returns packets that exceeded the maximum retry count.
func (pt *PendingTracker) GetLost() []uint64 {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	var lost []uint64
	for seq, entry := range pt.pending {
		if entry.Retries >= MaxRetries && time.Now().After(entry.NextRetry) {
			lost = append(lost, seq)
		}
	}

	for _, seq := range lost {
		delete(pt.pending, seq)
	}

	return lost
}

// IsAcked checks if a packet was acknowledged.
func (pt *PendingTracker) IsAcked(seq uint64) bool {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	return pt.acked[seq]
}
