package crypto

import "github.com/DesKaOne/DesKaEcosystem/internal/core/types"

type Hasher interface {
	Hash(data []byte) types.Hash
}

type Signer interface {
	Sign(message []byte) ([]byte, error)
}

type Verifier interface {
	Verify(message, signature, publicKey []byte) bool
}
