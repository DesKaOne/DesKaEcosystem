package transaction

import (
	"crypto/sha256"

	"github.com/DesKaOne/DesKaEcosystem/internal/core/types"
)

func Hash(tx Transaction) types.Hash {
	// Temporary deterministic representation for development tests.
	// Replace with the frozen canonical transaction encoding before protocol freeze.
	h := sha256.New()
	h.Write([]byte{byte(tx.Version >> 8), byte(tx.Version)})
	h.Write([]byte(tx.ChainID.String()))
	h.Write([]byte(tx.Sender))
	h.Write([]byte(tx.Recipient))
	return sha256.Sum256(h.Sum(nil))
}
