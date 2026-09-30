package consensus

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/mempool"
)

func TestLocalBlockProducerBuildsExecutedCandidateWithoutCanonicalMutation(t *testing.T) {
	roundState, err := NewRoundState(1, 1001, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	canonical := state.New()
	proposer := []byte("validator-a")
	ctx := BlockProductionContext{
		State:        roundState,
		PreviousHash: types.Hash{9},
		Proposer:     proposer,
	}
	pool := mempool.New(mempool.Config{MaxTransactions: 10})
	rules := block.ExecutionRules{ChainID: 1001, ProtocolVersion: 1}

	producer, err := NewLocalBlockProducer(pool, canonical, rules, func() int64 { return 12345 })
	if err != nil {
		t.Fatal(err)
	}
	before := canonical.Root()
	candidate, err := producer.ProduceBlock(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if candidate.Header.Height != 1 {
		t.Fatalf("height=%d, want 1", candidate.Header.Height)
	}
	if candidate.Header.Timestamp != 12345 {
		t.Fatalf("timestamp=%d, want 12345", candidate.Header.Timestamp)
	}
	if candidate.Header.TransactionsRoot == (types.Hash{}) {
		t.Fatal("transactions root is zero")
	}
	if candidate.Header.StateRoot != canonical.Root() {
		t.Fatal("empty candidate state root differs from canonical root")
	}
	if got := canonical.Root(); got != before {
		t.Fatal("canonical state mutated during production")
	}
}

func TestBuildCandidateFromTransactionsRejectsRuleContextMismatch(t *testing.T) {
	roundState, err := NewRoundState(1, 1001, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = BuildCandidateFromTransactions(
		BlockProductionContext{
			State:        roundState,
			PreviousHash: types.Hash{1},
			Proposer:     []byte("validator-a"),
		},
		nil,
		state.New(),
		block.ExecutionRules{ChainID: 1002, ProtocolVersion: 1},
		1,
	)
	if err == nil {
		t.Fatal("BuildCandidateFromTransactions() error = nil, want context mismatch")
	}
}
