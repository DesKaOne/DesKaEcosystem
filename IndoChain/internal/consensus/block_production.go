package consensus

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidBlockProductionContext = errors.New("invalid block production context")
	ErrBlockProductionContextMismatch = errors.New("block production context mismatch")
)

// BlockProductionContext carries the consensus context required to build the
// next block. It deliberately does not define transaction selection, fee
// policy, execution, or persistence.
type BlockProductionContext struct {
	State        RoundState
	PreviousHash types.Hash
	Proposer     []byte
}

// BlockProducer is the integration boundary between consensus and block
// construction. Production policy remains outside this interface.
type BlockProducer interface {
	ProduceBlock(ctx BlockProductionContext) (block.Block, error)
}

// ValidateProducedBlock checks that a candidate block belongs to the current
// consensus height/context and returns its development block identifier.
// The returned hash is an opaque proposal payload for the current development
// protocol; canonical block serialization remains unfrozen.
func ValidateProducedBlock(ctx BlockProductionContext, candidate block.Block) (types.Hash, error) {
	if err := ctx.State.Validate(); err != nil {
		return types.Hash{}, err
	}
	if len(ctx.Proposer) == 0 {
		return types.Hash{}, ErrInvalidBlockProductionContext
	}
	if candidate.Header.Version != ctx.State.ProtocolVersion ||
		candidate.Header.ChainID != ctx.State.ChainID {
		return types.Hash{}, fmt.Errorf("%w: protocol or chain context mismatch", ErrBlockProductionContextMismatch)
	}
	if candidate.Header.Height != ctx.State.Height+1 {
		return types.Hash{}, fmt.Errorf("%w: expected height %d got %d",
			ErrBlockProductionContextMismatch, ctx.State.Height+1, candidate.Header.Height)
	}
	if candidate.Header.PreviousHash != ctx.PreviousHash {
		return types.Hash{}, fmt.Errorf("%w: previous hash mismatch", ErrBlockProductionContextMismatch)
	}
	if !bytes.Equal(candidate.Header.Proposer, ctx.Proposer) {
		return types.Hash{}, fmt.Errorf("%w: proposer mismatch", ErrBlockProductionContextMismatch)
	}
	if err := block.ValidateTransactionsRoot(candidate); err != nil {
		return types.Hash{}, err
	}
	hash, err := block.Hash(candidate)
	if err != nil {
		return types.Hash{}, err
	}
	return hash, nil
}
