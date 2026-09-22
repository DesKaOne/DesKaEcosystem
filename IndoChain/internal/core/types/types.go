package types

import "encoding/hex"

type Hash [32]byte

func (h Hash) Bytes() []byte {
	b := make([]byte, len(h))
	copy(b, h[:])
	return b
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

type Height uint64
type Nonce uint64
type ChainID uint64
type ProtocolVersion uint16
