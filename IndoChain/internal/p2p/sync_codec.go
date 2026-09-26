package p2p

import (
	"encoding/binary"
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var ErrInvalidSyncPayload = errors.New("invalid sync payload")

// EncodeBlockRequest is the development wire codec for BlockRequest.
// It is intentionally separate from the canonical protocol serialization.
func EncodeBlockRequest(req BlockRequest, maxLimit uint64) ([]byte, error) {
	if err := ValidateBlockRequest(req, maxLimit); err != nil {
		return nil, err
	}
	out := make([]byte, 16)
	binary.BigEndian.PutUint64(out[:8], uint64(req.FromHeight))
	binary.BigEndian.PutUint64(out[8:], req.Limit)
	return out, nil
}

func DecodeBlockRequest(data []byte, maxLimit uint64) (BlockRequest, error) {
	if len(data) != 16 {
		return BlockRequest{}, ErrInvalidSyncPayload
	}
	req := BlockRequest{
		FromHeight: types.Height(binary.BigEndian.Uint64(data[:8])),
		Limit:      binary.BigEndian.Uint64(data[8:]),
	}
	if err := ValidateBlockRequest(req, maxLimit); err != nil {
		return BlockRequest{}, err
	}
	return req, nil
}
