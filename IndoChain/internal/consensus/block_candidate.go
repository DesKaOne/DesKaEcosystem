package consensus

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
)

var (
	ErrNilBlockProductionState = errors.New("nil block production state")
	ErrInvalidBlockTimestamp   = errors.New("invalid block timestamp")
)

// BlockCandidateInput contains the deterministic inputs required to construct
// the next development block candidate. Transaction selection remains outside
// this boundary.
type BlockCandidateInput struct {
	Context           BlockProductionContext
	Timestamp         int64
	Transactions      []any
	ConsensusEvidence []byte
	Rules             block.ExecutionRules
}

// BuildBlockCandidate executes the supplied transaction sequence against a
// snapshot and constructs a block whose transaction and state roots describe
// that resulting candidate state. It does not mutate the caller's state.
func BuildBlockCandidate(input BlockCandidateInput, canonicalState *state.State) (block.Block, error) {
	if canonicalState == nil {
		return block.Block{}, ErrNilBlockProductionState
	}
	if err := input.Context.State.Validate(); err != nil {
		return block.Block{}, err
	}
	if len(input.Context.Proposer) == 0 {
		return block.Block{}, ErrInvalidBlockProductionContext
	}
	if input.Timestamp < 0 {
		return block.Block{}, ErrInvalidBlockTimestamp
	}
	if input.Rules.ChainID != input.Context.State.ChainID ||
		input.Rules.ProtocolVersion != input.Context.State.ProtocolVersion {
		return block.Block{}, fmt.Errorf("%w: execution rules context mismatch", ErrBlockProductionContextMismatch)
	}

	working := canonicalState.Snapshot()
	for index, rawTx := range input.Transactions {
		tx, ok := rawTx.(interface{})
		if !ok {
			return block.Block{}, fmt.Errorf("transaction %d: invalid value", index)
		}
		_ = tx
	}

	for index, rawTx := range input.Transactions {
		tx, ok := rawTx.(transaction.Transaction)
		if !ok {
			return block.Block{}, fmt.Errorf("%w at index %d", block.ErrInvalidBlockTransaction, index)
		}
		if err := state.ApplyTransaction(working, tx, input.Rules.Transaction); err != nil {
			return block.Block{}, fmt.Errorf("transaction %d: %w", index, err)
		}
	}

	txsRoot, err := block.TransactionsRoot(input.Transactions)
	if err != nil {
		return block.Block{}, err
	}

	candidate := block.Block{
		Header: block.Header{
			Version:           input.Context.State.ProtocolVersion,
			ChainID:            input.Context.State.ChainID,
			Height:            input.Context.State.Height + 1,
			Timestamp:         input.Timestamp,
			PreviousHash:      input.Context.PreviousHash,
			TransactionsRoot:  txsRoot,
			StateRoot:         working.Root(),
			Proposer:           append([]byte(nil), input.Context.Proposer...),
			ConsensusEvidence: append([]byte(nil), input.ConsensusEvidence...),
		},
		Transactions: cloneBlockTransactions(input.Transactions),
	}
	if _, err := ValidateProducedBlock(input.Context, candidate); err != nil {
		return block.Block{}, err
	}
	return candidate, nil
}

func cloneBlockTransactions(txs []any) []any {
	out := make([]any, len(txs))
	for i, raw := range txs {
		switch tx := raw.(type) {
		case transaction.Transaction:
			tx.Sender = append([]byte(nil), tx.Sender...)
			tx.Recipient = append([]byte(nil), tx.Recipient...)
			tx.Data = append([]byte(nil), tx.Data...)
			tx.Signature = append([]byte(nil), tx.Signature...)
			out[i] = tx
		default:
			out[i] = raw
		}
	}
	return out
}
