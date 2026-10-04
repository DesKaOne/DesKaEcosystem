package crypto

import (
	"crypto/sha256"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type SHA256Hasher struct{}

func (SHA256Hasher) Hash(data []byte) types.Hash {
	return sha256.Sum256(data)
}
