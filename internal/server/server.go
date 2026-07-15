package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/telecom/udp-protocol/internal/protocol"
)

const (
	timeFormat = "15:04:05.000000"
)

// Config holds server configuration.
type Config struct {
	ListenAddr string
	Workers    int
}

// Stats holds server statistics.
type Stats struct {
	Received   uint64
	Valid      uint64
	Invalid    uint64
	Duplicates uint64
	ACKSent    uint64
	NACKSent   uint64
}

// Server is a UDP server that validates packets and sends acknowledgments.
type Server struct {
	cfg      Config
	conn     *net.UDPConn
	stats    Stats
	seenSeqs sync.Map
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// New creates a new Server instance.
func New(cfg Config) *Server {
	if cfg.Workers <= 0 {
		cfg.Workers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins listening and processing packets.
func (s *Server) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("resolve addr: %w", err)
	}

	s.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}

	if err := s.conn.SetReadBuffer(4 * 1024 * 1024); err != nil {
		log.Printf("Warning: failed to set read buffer: %v", err)
	}

	log.Printf("Server started on %s (workers: %d)", s.cfg.ListenAddr, s.cfg.Workers)

	packets := make(chan *rawPacket, 1000)

	for i := 0; i < s.cfg.Workers; i++ {
		s.wg.Add(1)
		go s.worker(i, packets)
	}

	s.wg.Add(1)
	go s.receiver(packets)

	<-s.ctx.Done()

	s.conn.Close()
	close(packets)
	s.wg.Wait()
	s.printStats()

	return nil
}

func (s *Server) Stop() {
	s.cancel()
}

type rawPacket struct {
	data []byte
	addr *net.UDPAddr
}

func (s *Server) receiver(packets chan<- *rawPacket) {
	defer s.wg.Done()

	buf := make([]byte, protocol.UDPPracticalLimit+protocol.MaxDataSize)

	for {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		select {
		case packets <- &rawPacket{data: data, addr: addr}:
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Server) worker(id int, packets <-chan *rawPacket) {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker %d recovered from panic: %v", id, r)
		}
	}()

	for raw := range packets {
		s.processPacket(raw)
	}
}

func (s *Server) processPacket(raw *rawPacket) {
	receivedAt := time.Now()
	atomic.AddUint64(&s.stats.Received, 1)

	p, err := protocol.Unmarshal(raw.data)
	if err != nil {
		atomic.AddUint64(&s.stats.Invalid, 1)
		return
	}

	if _, loaded := s.seenSeqs.LoadOrStore(p.SeqNumber, true); loaded {
		atomic.AddUint64(&s.stats.Duplicates, 1)
	}

	if protocol.VerifyChecksum(p) {
		atomic.AddUint64(&s.stats.Valid, 1)
		atomic.AddUint64(&s.stats.ACKSent, 1)
		s.sendAck(raw.addr, p.SeqNumber, protocol.StatusACK)

		log.Printf("[SERVER] Seq=%d FormedAt=%s ReceivedAt=%s Status=VALID",
			p.SeqNumber,
			time.Unix(0, p.Timestamp).Format(timeFormat),
			receivedAt.Format(timeFormat),
		)
	} else {
		atomic.AddUint64(&s.stats.Invalid, 1)
		atomic.AddUint64(&s.stats.NACKSent, 1)
		s.sendAck(raw.addr, p.SeqNumber, protocol.StatusNACK)

		log.Printf("[SERVER] Seq=%d FormedAt=%s ReceivedAt=%s Status=CORRUPTED",
			p.SeqNumber,
			time.Unix(0, p.Timestamp).Format(timeFormat),
			receivedAt.Format(timeFormat),
		)
	}
}

func (s *Server) sendAck(addr *net.UDPAddr, seqNumber uint64, status protocol.StatusCode) {
	ack := &protocol.AckMessage{
		SeqNumber: seqNumber,
		Status:    status,
	}

	data, err := protocol.MarshalACK(ack)
	if err != nil {
		log.Printf("Marshal ACK error: %v", err)
		return
	}

	if _, err := s.conn.WriteToUDP(data, addr); err != nil {
		log.Printf("Send ACK error: %v", err)
	}
}

func (s *Server) printStats() {
	log.Println("=== Server Statistics ===")
	log.Printf("Received:   %d", atomic.LoadUint64(&s.stats.Received))
	log.Printf("Valid:      %d", atomic.LoadUint64(&s.stats.Valid))
	log.Printf("Invalid:    %d", atomic.LoadUint64(&s.stats.Invalid))
	log.Printf("Duplicates: %d", atomic.LoadUint64(&s.stats.Duplicates))
	log.Printf("ACK sent:   %d", atomic.LoadUint64(&s.stats.ACKSent))
	log.Printf("NACK sent:   %d", atomic.LoadUint64(&s.stats.NACKSent))
}
