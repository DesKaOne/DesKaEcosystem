package transaction

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"

// Transaction is the development transaction model.
// Sender and Recipient use the protocol Address abstraction so the final
// wire format can remain independent from a specific address encoding.
type Transaction struct {
	Version   types.ProtocolVersion
	ChainID   types.ChainID
	Nonce     types.Nonce
	Sender    types.Address
	Recipient types.Address
	Value     uint64
	GasLimit  uint64
	Data      []byte
	Signature []byte
}
