package protocol

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"log"
)

// ComputeChecksum calculates SHA256 over all packet fields except Checksum itself.
func ComputeChecksum(p *Packet) [32]byte {
	var buf bytes.Buffer

	if err := binary.Write(&buf, binary.BigEndian, p.SeqNumber); err != nil {
		log.Printf("checksum: write seq number: %v", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, p.Timestamp); err != nil {
		log.Printf("checksum: write timestamp: %v", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, p.DataLength); err != nil {
		log.Printf("checksum: write data length: %v", err)
	}
	if err := binary.Write(&buf, binary.BigEndian, p.Data); err != nil {
		log.Printf("checksum: write data: %v", err)
	}

	return sha256.Sum256(buf.Bytes())
}

// VerifyChecksum checks the integrity of a packet.
func VerifyChecksum(p *Packet) bool {
	computed := ComputeChecksum(p)
	return bytes.Equal(computed[:], p.Checksum[:])
}
