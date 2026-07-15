package client

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telecom/udp-protocol/internal/protocol"
)

// Config holds client configuration.
type Config struct {
	ServerAddr  string
	PacketCount uint64
	Workers     int
	Timeout     time.Duration
}

// Stats holds client statistics.
type Stats struct {
	Sent   uint64
	Acked  uint64
	Nacked uint64
	Lost   uint64
}

// Client is a UDP client that generates packets and processes acknowledgments.
type Client struct {
	cfg            Config
	conn           *net.UDPConn
	server         *net.UDPAddr
	seq            uint64
	stats          Stats
	printer        *OrderedPrinter
	pending        *PendingTracker
	formedAt       sync.Map
	generatorsDone chan struct{}
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// New creates a new Client instance.
func New(cfg Config) (*Client, error) {
	serverAddr, err := net.ResolveUDPAddr("udp", cfg.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve server addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, fmt.Errorf("listen udp: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)

	return &Client{
		cfg:            cfg,
		conn:           conn,
		server:         serverAddr,
		printer:        NewOrderedPrinter(),
		pending:        NewPendingTracker(),
		generatorsDone: make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}, nil
}

// Run starts the client.
func (c *Client) Run() error {
	defer c.conn.Close()
	defer c.cancel()

	log.Printf("Client starting: server=%s packets=%d workers=%d",
		c.cfg.ServerAddr, c.cfg.PacketCount, c.cfg.Workers)

	c.wg.Add(1)
	go c.ackReceiver()

	c.wg.Add(1)
	go c.retransmitter()

	startTime := time.Now()

	c.wg.Add(1)
	go c.generatePackets()

	c.wg.Wait()

	c.printer.MarkDone()
	c.printer.WaitForCompletion(c.cfg.PacketCount)

	elapsed := time.Since(startTime)
	c.printStats(elapsed)

	return nil
}

func (c *Client) Stop() {
	c.cancel()
}

func (c *Client) generatePackets() {
	defer c.wg.Done()

	var genWG sync.WaitGroup

	for i := 0; i < c.cfg.Workers; i++ {
		genWG.Add(1)
		go c.worker(i, &genWG)
	}

	genWG.Wait()
	close(c.generatorsDone)
	log.Printf("All generators finished")
}

func (c *Client) worker(id int, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker %d recovered from panic: %v", id, r)
		}
	}()

	rng := rand.New(rand.NewPCG(uint64(id), uint64(time.Now().UnixNano())))

	for {
		seq := atomic.AddUint64(&c.seq, 1)
		if seq > c.cfg.PacketCount {
			return
		}

		dataLen := int(seq) + rng.IntN(int(seq))
		if dataLen > protocol.MaxDataSize {
			dataLen = protocol.MaxDataSize
		}
		if dataLen < 1 {
			dataLen = 1
		}
		data := make([]byte, dataLen)
		if _, err := crand.Read(data); err != nil {
			log.Printf("Worker %d: generate random data: %v", id, err)
			continue
		}

		p, err := protocol.NewPacket(seq, time.Now().UnixNano(), data)
		if err != nil {
			log.Printf("Worker %d: create packet %d: %v", id, seq, err)
			continue
		}

		c.formedAt.Store(seq, time.Unix(0, p.Timestamp))

		if err := c.sendPacket(p); err != nil {
			log.Printf("Worker %d: send packet %d: %v", id, seq, err)
		} else {
			c.pending.Add(p.SeqNumber, p)
		}

		time.Sleep(100 * time.Microsecond)
	}
}

func (c *Client) sendPacket(p *protocol.Packet) error {
	data, err := p.Marshal()
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if _, err := c.conn.WriteToUDP(data, c.server); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	atomic.AddUint64(&c.stats.Sent, 1)
	return nil
}

func (c *Client) ackReceiver() {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ACK receiver recovered from panic: %v", r)
		}
	}()

	buf := make([]byte, protocol.ACKPacketSize)

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		if err := c.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			log.Printf("set read deadline: %v", err)
		}

		n, _, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if c.ctx.Err() != nil {
				return
			}
			continue
		}

		ack, err := protocol.UnmarshalACK(buf[:n])
		if err != nil {
			continue
		}

		switch ack.Status {
		case protocol.StatusACK:
			if !c.pending.IsAcked(ack.SeqNumber) {
				atomic.AddUint64(&c.stats.Acked, 1)
			}
			c.pending.MarkAcked(ack.SeqNumber)
		case protocol.StatusNACK:
			atomic.AddUint64(&c.stats.Nacked, 1)
		}

		formedAt, _ := c.formedAt.Load(ack.SeqNumber)
		var formedTime time.Time
		if formedAt != nil {
			formedTime = formedAt.(time.Time)
		}

		c.printer.AddResult(ack.SeqNumber, &PrintEntry{
			SeqNumber:  ack.SeqNumber,
			FormedAt:   formedTime,
			ReceivedAt: time.Now(),
			Status:     ack.Status,
		})
	}
}

func (c *Client) retransmitter() {
	defer c.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Retransmitter recovered from panic: %v", r)
		}
	}()

	ticker := time.NewTicker(RetransmitInterval)
	defer ticker.Stop()

	generatorsDone := false

	for {
		if generatorsDone && c.pending.IsEmpty() {
			c.cancel()
			return
		}

		select {
		case <-c.ctx.Done():
			return
		case <-c.generatorsDone:
			generatorsDone = true
		case <-ticker.C:
			expired := c.pending.GetExpired()
			for _, entry := range expired {
				if err := c.sendPacket(entry.Packet); err != nil {
					log.Printf("Retransmit Seq=%d: %v", entry.Packet.SeqNumber, err)
				}
			}

			losts := c.pending.GetLost()
			for _, seq := range losts {
				atomic.AddUint64(&c.stats.Lost, 1)

				formedAt, _ := c.formedAt.Load(seq)
				var formedTime time.Time
				if formedAt != nil {
					formedTime = formedAt.(time.Time)
				}

				c.printer.AddResult(seq, &PrintEntry{
					SeqNumber:  seq,
					FormedAt:   formedTime,
					ReceivedAt: time.Now(),
					Status:     protocol.StatusLOST,
				})
			}
		}
	}
}

func (c *Client) printStats(elapsed time.Duration) {
	sent := atomic.LoadUint64(&c.stats.Sent)
	acked := atomic.LoadUint64(&c.stats.Acked)
	nacked := atomic.LoadUint64(&c.stats.Nacked)
	lost := atomic.LoadUint64(&c.stats.Lost)

	log.Println("=== Client Statistics ===")
	log.Printf("Expected: %d", c.cfg.PacketCount)
	log.Printf("Sent:     %d", sent)
	log.Printf("Acked:    %d", acked)
	log.Printf("Nacked:   %d", nacked)
	log.Printf("Lost:     %d", lost)
	log.Printf("Elapsed:  %v", elapsed)

	lossRate := float64(lost) / float64(c.cfg.PacketCount) * 100
	log.Printf("Loss:     %.2f%%", lossRate)
}
