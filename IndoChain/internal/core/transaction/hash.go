package transaction

import (
	"crypto/sha256"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func Hash(tx Transaction) types.Hash {
	// Temporary deterministic representation for development tests.
	// Replace with the frozen canonical transaction encoding before protocol freeze.
	h := sha256.New()
	h.Write([]byte{byte(tx.Version >> 8), byte(tx.Version)})
	h.Write([]byte{byte(tx.ChainID >> 56), byte(tx.ChainID >> 48), byte(tx.ChainID >> 40), byte(tx.ChainID >> 32), byte(tx.ChainID >> 24), byte(tx.ChainID >> 16), byte(tx.ChainID >> 8), byte(tx.ChainID)})
	h.Write([]byte(tx.Sender))
	h.Write([]byte(tx.Recipient))
	return sha256.Sum256(h.Sum(nil))
}
