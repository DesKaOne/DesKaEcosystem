package encoding

import (
	"encoding/binary"
	"errors"
)

var ErrInvalidData = errors.New("invalid canonical data")

// Uint64 encodes an unsigned integer in fixed-width big-endian form.
// This helper is intentionally small and deterministic; it is not yet the
// complete wire format for protocol objects.
func Uint64(v uint64) []byte {
	out := make([]byte, 8)
	binary.BigEndian.PutUint64(out, v)
	return out
}

func Uint64From(data []byte) (uint64, error) {
	if len(data) != 8 {
		return 0, ErrInvalidData
	}
	return binary.BigEndian.Uint64(data), nil
}
