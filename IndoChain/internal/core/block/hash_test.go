package block

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestHashDeterministic(t *testing.T) {
	b := Block{
		Header: Header{
			Version: 1,
			ChainID: 1001,
			Height: 1,
			Timestamp: 100,
			PreviousHash: types.Hash{1},
			TransactionsRoot: types.Hash{2},
			StateRoot: types.Hash{3},
			Proposer: []byte{4, 5},
			ConsensusEvidence: []byte{6, 7},
		},
	}
	first, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("hash changed: %s vs %s", first, second)
	}
}

func TestHashChangesWhenHeaderChanges(t *testing.T) {
	b := Block{Header: Header{Version: 1, ChainID: 1001, Height: 1}}
	first, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}

	b.Header.Height++
	second, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("block hash did not change after header mutation")
	}
}

func TestHashDoesNotDependOnTransactionListDirectly(t *testing.T) {
	b := Block{Header: Header{Version: 1, ChainID: 1001, Height: 1}}
	first, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}

	b.Transactions = []any{"not encoded into header hash"}
	second, err := Hash(b)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("block hash unexpectedly depended on raw transaction list")
	}
}
