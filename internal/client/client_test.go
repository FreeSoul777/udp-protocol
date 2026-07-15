package client

import (
	"testing"
	"time"

	"github.com/telecom/udp-protocol/internal/protocol"
)

func TestOrderedPrinterBasic(t *testing.T) {
	op := NewOrderedPrinter()

	now := time.Now()
	op.AddResult(2, &PrintEntry{
		SeqNumber:  2,
		FormedAt:   now,
		ReceivedAt: now,
		Status:     1,
	})

	op.AddResult(1, &PrintEntry{
		SeqNumber:  1,
		FormedAt:   now,
		ReceivedAt: now,
		Status:     1,
	})

	op.MarkDone()
	op.WaitForCompletion(2)
}

func TestPendingTrackerAdd(t *testing.T) {
	pt := NewPendingTracker()

	p := &protocol.Packet{SeqNumber: 1}
	pt.Add(1, p)

	if len(pt.pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(pt.pending))
	}
}

func TestPendingTrackerMarkAcked(t *testing.T) {
	pt := NewPendingTracker()

	p := &protocol.Packet{SeqNumber: 1}
	pt.Add(1, p)
	pt.MarkAcked(1)

	if len(pt.pending) != 0 {
		t.Errorf("Expected 0 pending after ack, got %d", len(pt.pending))
	}

	if !pt.IsAcked(1) {
		t.Error("Expected IsAcked true")
	}
}
