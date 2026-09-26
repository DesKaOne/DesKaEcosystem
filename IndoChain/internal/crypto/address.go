package crypto

import (
	"crypto/sha256"
	"errors"
)

var ErrInvalidAddress = errors.New("invalid indochain address")

// AddressCodec is a development-only Base58 address codec.
// Prefix, version layout, checksum algorithm, and payload semantics are not frozen.
type AddressCodec struct {
	Prefix  string
	Version byte
}

func (c AddressCodec) Encode(payload []byte) string {
	body := make([]byte, 1+len(payload))
	body[0] = c.Version
	copy(body[1:], payload)
	checksum := addressChecksum(body)
	encoded := Base58Encode(append(body, checksum...))
	return c.Prefix + encoded
}

func (c AddressCodec) Decode(address string) ([]byte, error) {
	if len(address) <= len(c.Prefix) || address[:len(c.Prefix)] != c.Prefix {
		return nil, ErrInvalidAddress
	}
	raw, err := Base58Decode(address[len(c.Prefix):])
	if err != nil || len(raw) < 5 {
		return nil, ErrInvalidAddress
	}
	body, checksum := raw[:len(raw)-4], raw[len(raw)-4:]
	if body[0] != c.Version {
		return nil, ErrInvalidAddress
	}
	expected := addressChecksum(body)
	if string(checksum) != string(expected) {
		return nil, ErrInvalidAddress
	}
	payload := make([]byte, len(body)-1)
	copy(payload, body[1:])
	return payload, nil
}

func addressChecksum(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	out := make([]byte, 4)
	copy(out, second[:4])
	return out
}
