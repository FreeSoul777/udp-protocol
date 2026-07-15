package protocol

import (
	"bytes"
	"testing"
	"time"
)

func TestMarshalUnmarshal(t *testing.T) {
	original := &Packet{
		SeqNumber:  42,
		Timestamp:  time.Now().UnixNano(),
		DataLength: 100,
		Data:       bytes.Repeat([]byte{0xAB}, 100),
	}
	original.Checksum = ComputeChecksum(original)

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	restored, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if restored.SeqNumber != original.SeqNumber {
		t.Errorf("SeqNumber mismatch: got %d, want %d", restored.SeqNumber, original.SeqNumber)
	}
	if restored.Timestamp != original.Timestamp {
		t.Errorf("Timestamp mismatch")
	}
	if restored.DataLength != original.DataLength {
		t.Errorf("DataLength mismatch")
	}
	if !bytes.Equal(restored.Data, original.Data) {
		t.Errorf("Data mismatch")
	}
	if !bytes.Equal(restored.Checksum[:], original.Checksum[:]) {
		t.Errorf("Checksum mismatch")
	}
}

func TestChecksumVerification(t *testing.T) {
	p := &Packet{
		SeqNumber:  1,
		Timestamp:  time.Now().UnixNano(),
		DataLength: 50,
		Data:       bytes.Repeat([]byte{0xCD}, 50),
	}
	p.Checksum = ComputeChecksum(p)

	if !VerifyChecksum(p) {
		t.Error("Valid checksum verification failed")
	}

	p.Data[0] ^= 0xFF

	if VerifyChecksum(p) {
		t.Error("Corrupted data passed checksum verification")
	}
}

func TestACKMarshalUnmarshal(t *testing.T) {
	original := &AckMessage{
		SeqNumber: 12345,
		Status:    StatusACK,
	}

	data, err := MarshalACK(original)
	if err != nil {
		t.Fatalf("MarshalACK failed: %v", err)
	}

	restored, err := UnmarshalACK(data)
	if err != nil {
		t.Fatalf("UnmarshalACK failed: %v", err)
	}

	if restored.SeqNumber != original.SeqNumber {
		t.Errorf("SeqNumber mismatch")
	}
	if restored.Status != original.Status {
		t.Errorf("Status mismatch: got %v, want %v", restored.Status, original.Status)
	}
}

func TestMaxDataSize(t *testing.T) {
	p := &Packet{
		SeqNumber:  1,
		Timestamp:  time.Now().UnixNano(),
		DataLength: MaxDataSize,
		Data:       bytes.Repeat([]byte{0xEF}, MaxDataSize),
	}
	p.Checksum = ComputeChecksum(p)

	data, err := p.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal max size packet: %v", err)
	}

	if len(data) > UDPPracticalLimit {
		t.Errorf("Packet size %d exceeds practical limit %d", len(data), UDPPracticalLimit)
	}
}

func TestUnmarshalShortPacket(t *testing.T) {
	shortData := make([]byte, 10)
	_, err := Unmarshal(shortData)
	if err == nil {
		t.Error("Expected error for short packet, got nil")
	}
}

func TestNewPacket(t *testing.T) {
	seqNumber := uint64(42)
	timestamp := time.Now().UnixNano()
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x05}

	p, err := NewPacket(seqNumber, timestamp, data)
	if err != nil {
		t.Fatalf("NewPacket failed: %v", err)
	}

	if p.SeqNumber != seqNumber {
		t.Errorf("SeqNumber mismatch: got %d, want %d", p.SeqNumber, seqNumber)
	}
	if p.Timestamp != timestamp {
		t.Errorf("Timestamp mismatch")
	}
	if p.DataLength != uint32(len(data)) {
		t.Errorf("DataLength mismatch: got %d, want %d", p.DataLength, len(data))
	}
	if !bytes.Equal(p.Data, data) {
		t.Errorf("Data mismatch")
	}

	expectedChecksum := ComputeChecksum(p)
	if !bytes.Equal(p.Checksum[:], expectedChecksum[:]) {
		t.Errorf("Checksum mismatch: NewPacket didn't compute checksum correctly")
	}

	if !VerifyChecksum(p) {
		t.Error("VerifyChecksum failed for valid packet")
	}
}

func TestNewPacketTooLarge(t *testing.T) {
	data := make([]byte, MaxDataSize+1)
	_, err := NewPacket(1, time.Now().UnixNano(), data)
	if err == nil {
		t.Error("Expected error for oversized data, got nil")
	}
}
