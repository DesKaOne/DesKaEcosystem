package transaction

import (
    "crypto/sha256"

    "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

// Hash returns a development transaction identifier derived from the complete
// signed transaction encoding. The canonical hash algorithm remains subject
// to protocol freeze.
func Hash(tx Transaction) types.Hash {
    signed, err := SignedBytes(tx)
    if err != nil {
        return types.Hash{}
    }
    return sha256.Sum256(signed)
}
