package devnet

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"

type Genesis struct {
	ChainID            types.ChainID
	NetworkProfile     string
	ProtocolVersion    types.ProtocolVersion
	Timestamp          int64
	InitialValidators  [][]byte
	InitialAccounts    [][]byte
	InitialAllocations map[string]uint64
}
