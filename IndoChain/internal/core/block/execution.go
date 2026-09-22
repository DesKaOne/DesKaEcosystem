package block

import (
	"errors"
	"fmt"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrNilState          = errors.New("nil execution state")
	ErrWrongChainID      = errors.New("invalid block chain id")
	ErrWrongVersion      = errors.New("invalid block protocol version")
	ErrHeightMismatch    = errors.New("invalid block height")
	ErrPreviousHash      = errors.New("invalid previous hash")
	ErrTransactionType   = errors.New("invalid block transaction type")
	ErrStateRootMismatch = errors.New("state root mismatch")
)

type ExecutionRules struct {
	ChainID         types.ChainID
	ProtocolVersion types.ProtocolVersion
	Transaction     state.ExecutionRules
}

func ValidateHeader(b Block, expectedHeight types.Height, expectedPreviousHash types.Hash, rules ExecutionRules) error {
	if b.Header.ChainID != rules.ChainID {
		return ErrWrongChainID
	}
	if b.Header.Version != rules.ProtocolVersion {
		return ErrWrongVersion
	}
	if b.Header.Height != expectedHeight {
		return ErrHeightMismatch
	}
	if b.Header.PreviousHash != expectedPreviousHash {
		return ErrPreviousHash
	}
	return nil
}

func ExecuteBlock(s *state.State, b Block, expectedHeight types.Height, expectedPreviousHash types.Hash, rules ExecutionRules) error {
	if s == nil {
		return ErrNilState
	}
	if err := ValidateHeader(b, expectedHeight, expectedPreviousHash, rules); err != nil {
		return err
	}

	working := s.Snapshot()
	for index, rawTx := range b.Transactions {
		tx, ok := rawTx.(transaction.Transaction)
		if !ok {
			return fmt.Errorf("%w at index %d", ErrTransactionType, index)
		}
		if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil {
			return fmt.Errorf("transaction %d: %w", index, err)
		}
	}

	if b.Header.StateRoot != (types.Hash{}) && b.Header.StateRoot != working.Root() {
		return ErrStateRootMismatch
	}

	s.Replace(working)
	return nil
}
