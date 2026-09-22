package block

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func ValidateBlockCommitments(b Block) error {
	if err := ValidateTransactionsRoot(b); err != nil {
		return err
	}
	if b.Header.StateRoot != (types.Hash{}) {
		// StateRoot is validated after execution against the resulting state.
		return nil
	}
	return nil
}

func ExampleValidateBlockCommitments() {
	b := Block{Header: Header{Version: 1, ChainID: 1001, Height: 1}}
	_ = ValidateBlockCommitments(b)
	// Output:
	_ = state.New
	_ = transaction.Transaction{}
}
