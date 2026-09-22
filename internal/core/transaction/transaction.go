package transaction

import "github.com/DesKaOne/DesKaEcosystem/internal/core/types"

type Transaction struct {
	Version  types.ProtocolVersion
	ChainID  types.ChainID
	Nonce    types.Nonce
	Sender   []byte
	Recipient []byte
	Value    uint64
	GasLimit uint64
	Data     []byte
	Signature []byte
}
