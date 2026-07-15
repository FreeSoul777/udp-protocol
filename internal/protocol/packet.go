package protocol

// Package protocol implements a binary protocol for reliable data transfer over UDP.
//
// Packet format (big-endian):
//   SeqNumber  uint64    (8 bytes)
//   Timestamp  int64     (8 bytes, unix nano)
//   DataLength uint32    (4 bytes)
//   Data       []byte    (variable length)
//   Checksum   [32]byte  (SHA256 over all fields above)
//
// Maximum data size: 1348 bytes (MTU 1500 - IP/UDP headers - checksum).

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// UDPPracticalLimit is the practical UDP payload limit
	// (MTU 1500 - IP header 20 - UDP header 8 = 1472 bytes, rounded down for safety).
	UDPPracticalLimit = 1400

	// HeaderSize: SeqNumber(8) + Timestamp(8) + DataLength(4) = 20 bytes.
	HeaderSize = 20

	// ChecksumSize: SHA256 = 32 bytes.
	ChecksumSize = 32

	// MaxDataSize is the maximum data payload size.
	MaxDataSize = UDPPracticalLimit - HeaderSize - ChecksumSize

	// ACKPacketSize: SeqNumber(8) + Status(1) = 9 bytes.
	ACKPacketSize = 9
)

// StatusCode represents the delivery status of a packet.
type StatusCode uint8

const (
	StatusNACK StatusCode = iota
	StatusACK
	StatusLOST
)

// Packet is the data structure transmitted over UDP.
type Packet struct {
	SeqNumber  uint64
	Timestamp  int64
	DataLength uint32
	Data       []byte
	Checksum   [32]byte
}

// AckMessage is the delivery confirmation sent by the server.
type AckMessage struct {
	SeqNumber uint64
	Status    StatusCode
}

// NewPacket creates a new packet with computed checksum.
func NewPacket(seqNumber uint64, timestamp int64, data []byte) (*Packet, error) {
	if len(data) > MaxDataSize {
		return nil, fmt.Errorf("data size %d exceeds maximum %d", len(data), MaxDataSize)
	}

	p := &Packet{
		SeqNumber:  seqNumber,
		Timestamp:  timestamp,
		DataLength: uint32(len(data)),
		Data:       data,
	}
	p.Checksum = ComputeChecksum(p)
	return p, nil
}

// Marshal serializes the packet into binary format.
func (p *Packet) Marshal() ([]byte, error) {
	if len(p.Data) > MaxDataSize {
		return nil, fmt.Errorf("data size %d exceeds maximum %d", len(p.Data), MaxDataSize)
	}

	buf := new(bytes.Buffer)

	fields := []interface{}{
		p.SeqNumber,
		p.Timestamp,
		p.DataLength,
		p.Data,
		p.Checksum,
	}
	for _, v := range fields {
		if err := binary.Write(buf, binary.BigEndian, v); err != nil {
			return nil, fmt.Errorf("marshal field: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// Unmarshal deserializes binary data into a Packet.
func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < HeaderSize+ChecksumSize {
		return nil, fmt.Errorf("packet too short: %d bytes, minimum %d",
			len(data), HeaderSize+ChecksumSize)
	}

	buf := bytes.NewReader(data)
	p := &Packet{}

	if err := binary.Read(buf, binary.BigEndian, &p.SeqNumber); err != nil {
		return nil, fmt.Errorf("unmarshal seq number: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &p.Timestamp); err != nil {
		return nil, fmt.Errorf("unmarshal timestamp: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &p.DataLength); err != nil {
		return nil, fmt.Errorf("unmarshal data length: %w", err)
	}

	availableData := len(data) - HeaderSize - ChecksumSize
	if int(p.DataLength) > availableData {
		return nil, fmt.Errorf("declared data length %d exceeds available %d",
			p.DataLength, availableData)
	}

	p.Data = make([]byte, p.DataLength)
	if err := binary.Read(buf, binary.BigEndian, &p.Data); err != nil {
		return nil, fmt.Errorf("unmarshal data: %w", err)
	}
	if err := binary.Read(buf, binary.BigEndian, &p.Checksum); err != nil {
		return nil, fmt.Errorf("unmarshal checksum: %w", err)
	}

	return p, nil
}

// MarshalACK serializes an acknowledgment message.
func MarshalACK(ack *AckMessage) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := binary.Write(buf, binary.BigEndian, ack.SeqNumber); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, ack.Status); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalACK deserializes an acknowledgment message.
func UnmarshalACK(data []byte) (*AckMessage, error) {
	if len(data) < ACKPacketSize {
		return nil, fmt.Errorf("ack packet too short: %d bytes", len(data))
	}

	buf := bytes.NewReader(data)
	ack := &AckMessage{}

	if err := binary.Read(buf, binary.BigEndian, &ack.SeqNumber); err != nil {
		return nil, err
	}

	var status uint8
	if err := binary.Read(buf, binary.BigEndian, &status); err != nil {
		return nil, err
	}
	ack.Status = StatusCode(status)

	return ack, nil
}
