package transaction

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrInvalidTransactionEncoding = errors.New("invalid transaction encoding")

// EncodeDevelopment returns a deterministic development encoding.
// It is NOT the canonical wire format until serialization-spec-v0.1 is frozen.
func EncodeDevelopment(tx Transaction, includeSignature bool) ([]byte, error) {
	var b bytes.Buffer

	writeU16(&b, uint16(tx.Version))
	writeU64(&b, uint64(tx.ChainID))
	writeU64(&b, uint64(tx.Nonce))

	if err := writeBytes(&b, tx.Sender); err != nil {
		return nil, err
	}
	if err := writeBytes(&b, tx.Recipient); err != nil {
		return nil, err
	}

	writeU64(&b, tx.Value)
	writeU64(&b, tx.GasLimit)

	if err := writeBytes(&b, tx.Data); err != nil {
		return nil, err
	}

	if includeSignature {
		if err := writeBytes(&b, tx.Signature); err != nil {
			return nil, err
		}
	}

	return b.Bytes(), nil
}

func SigningBytes(tx Transaction) ([]byte, error) {
	return EncodeDevelopment(tx, false)
}

func SignedBytes(tx Transaction) ([]byte, error) {
	return EncodeDevelopment(tx, true)
}

func writeU16(b *bytes.Buffer, v uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], v)
	_, _ = b.Write(buf[:])
}

func writeU64(b *bytes.Buffer, v uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], v)
	_, _ = b.Write(buf[:])
}

func writeBytes(b *bytes.Buffer, data []byte) error {
	if uint64(len(data)) > uint64(^uint32(0)) {
		return fmt.Errorf("field too large: %d", len(data))
	}
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(data)))
	_, _ = b.Write(buf[:])
	_, _ = b.Write(data)
	return nil
}
