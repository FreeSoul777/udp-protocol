package client

import (
	"log"
	"sync"
	"time"

	"github.com/telecom/udp-protocol/internal/protocol"
)

const (
	timeFormat = "15:04:05.000000"
)

// PrintEntry holds information for ordered output.
type PrintEntry struct {
	SeqNumber  uint64
	FormedAt   time.Time
	ReceivedAt time.Time
	Status     protocol.StatusCode
}

// OrderedPrinter ensures results are printed in sequence number order.
type OrderedPrinter struct {
	mu          sync.Mutex
	cond        *sync.Cond
	nextToPrint uint64
	buffer      map[uint64]*PrintEntry
	done        bool
}

// NewOrderedPrinter creates a new OrderedPrinter.
func NewOrderedPrinter() *OrderedPrinter {
	op := &OrderedPrinter{
		nextToPrint: 1,
		buffer:      make(map[uint64]*PrintEntry),
	}
	op.cond = sync.NewCond(&op.mu)
	return op
}

// AddResult adds a result and prints all consecutive ready entries.
func (op *OrderedPrinter) AddResult(seqNumber uint64, entry *PrintEntry) {
	op.mu.Lock()
	defer op.mu.Unlock()

	if _, exists := op.buffer[seqNumber]; exists {
		return
	}

	op.buffer[seqNumber] = entry

	for {
		entry, ok := op.buffer[op.nextToPrint]
		if !ok {
			break
		}

		op.printEntry(entry)
		delete(op.buffer, op.nextToPrint)
		op.nextToPrint++
	}

	op.cond.Broadcast()
}

// MarkDone signals that no more packets are expected.
func (op *OrderedPrinter) MarkDone() {
	op.mu.Lock()
	op.done = true
	op.cond.Broadcast()
	op.mu.Unlock()
}

// WaitForCompletion waits until all expected packets are printed.
func (op *OrderedPrinter) WaitForCompletion(totalPackets uint64) {
	op.mu.Lock()
	defer op.mu.Unlock()

	for op.nextToPrint <= totalPackets && !op.done {
		op.cond.Wait()
		for {
			entry, ok := op.buffer[op.nextToPrint]
			if !ok {
				break
			}
			op.printEntry(entry)
			delete(op.buffer, op.nextToPrint)
			op.nextToPrint++
		}
	}

	for seq := op.nextToPrint; seq <= totalPackets; seq++ {
		if entry, ok := op.buffer[seq]; ok {
			op.printEntry(entry)
		} else {
			log.Printf("[CLIENT] Seq=%05d Status=LOST", seq)
		}
	}
}

func (op *OrderedPrinter) printEntry(entry *PrintEntry) {
	status := statusString(entry.Status)
	log.Printf("[CLIENT] Seq=%05d Formed=%s ACK=%s Status=%s",
		entry.SeqNumber,
		entry.FormedAt.Format(timeFormat),
		entry.ReceivedAt.Format(timeFormat),
		status,
	)
}

func statusString(s protocol.StatusCode) string {
	switch s {
	case protocol.StatusACK:
		return "DELIVERED"
	case protocol.StatusNACK:
		return "CORRUPTED"
	case protocol.StatusLOST:
		return "LOST"
	default:
		return "UNKNOWN"
	}
}
