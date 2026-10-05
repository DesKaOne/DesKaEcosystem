package block

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

// Hash returns the deterministic development identifier of a block header.
// The exact canonical block encoding remains subject to protocol freeze.
func Hash(b Block) (types.Hash, error) {
	var buf bytes.Buffer

	writeU16(&buf, uint16(b.Header.Version))
	writeU64(&buf, uint64(b.Header.ChainID))
	writeU64(&buf, uint64(b.Header.Height))
	writeI64(&buf, b.Header.Timestamp)
	_, _ = buf.Write(b.Header.PreviousHash[:])
	_, _ = buf.Write(b.Header.TransactionsRoot[:])
	_, _ = buf.Write(b.Header.StateRoot[:])

	if err := writeBytes(&buf, b.Header.Proposer); err != nil {
		return types.Hash{}, fmt.Errorf("proposer: %w", err)
	}
	if err := writeBytes(&buf, b.Header.ConsensusEvidence); err != nil {
		return types.Hash{}, fmt.Errorf("consensus evidence: %w", err)
	}

	return sha256.Sum256(buf.Bytes()), nil
}

func writeU16(buf *bytes.Buffer, value uint16) {
	var out [2]byte
	binary.BigEndian.PutUint16(out[:], value)
	_, _ = buf.Write(out[:])
}

func writeU64(buf *bytes.Buffer, value uint64) {
	var out [8]byte
	binary.BigEndian.PutUint64(out[:], value)
	_, _ = buf.Write(out[:])
}

func writeI64(buf *bytes.Buffer, value int64) {
	writeU64(buf, uint64(value))
}

func writeBytes(buf *bytes.Buffer, value []byte) error {
	if uint64(len(value)) > uint64(^uint32(0)) {
		return fmt.Errorf("field too large: %d", len(value))
	}
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = buf.Write(length[:])
	_, _ = buf.Write(value)
	return nil
}
