package block

import "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"

type Header struct {
	Version           types.ProtocolVersion
	ChainID           types.ChainID
	Height            types.Height
	Timestamp         int64
	PreviousHash      types.Hash
	TransactionsRoot  types.Hash
	StateRoot         types.Hash
	Proposer          []byte
	ConsensusEvidence []byte
}

type Block struct {
	Header       Header
	Transactions []any
}
