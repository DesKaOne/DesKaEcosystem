package consensus

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/transaction"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/state"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestBuildBlockCandidateBuildsDeterministicEmptyBlock(t *testing.T) {
	s := state.New()
	s.Set(types.Address{1}, state.Account{Balance: 100})

	ctx := BlockProductionContext{
		State: RoundState{
			ProtocolVersion: 1,
			ChainID:         1001,
			Height:          7,
			Round:           2,
			Phase:           PhaseProposal,
		},
		PreviousHash: types.Hash{9},
		Proposer:     []byte{3},
	}
	rules := block.ExecutionRules{ChainID: 1001, ProtocolVersion: 1}

	first, err := BuildBlockCandidate(BlockCandidateInput{
		Context:   ctx,
		Timestamp: 100,
		Rules:     rules,
	}, s)
	if err != nil {
		t.Fatalf("build first candidate: %v", err)
	}
	second, err := BuildBlockCandidate(BlockCandidateInput{
		Context:   ctx,
		Timestamp: 100,
		Rules:     rules,
	}, s)
	if err != nil {
		t.Fatalf("build second candidate: %v", err)
	}

	firstHash, err := block.Hash(first)
	if err != nil {
		t.Fatalf("hash first: %v", err)
	}
	secondHash, err := block.Hash(second)
	if err != nil {
		t.Fatalf("hash second: %v", err)
	}
	if firstHash != secondHash {
		t.Fatalf("candidate hash is not deterministic")
	}
	if first.Header.Height != 8 {
		t.Fatalf("expected height 8, got %d", first.Header.Height)
	}
	if first.Header.StateRoot != s.Root() {
		t.Fatalf("empty candidate changed state root")
	}
	if s.Root() != second.Header.StateRoot {
		t.Fatalf("canonical state was mutated")
	}
}

func TestBuildBlockCandidateRejectsExecutionContextMismatch(t *testing.T) {
	s := state.New()
	ctx := BlockProductionContext{
		State: RoundState{
			ProtocolVersion: 1,
			ChainID:         1001,
			Height:          0,
			Phase:           PhaseProposal,
		},
		Proposer: []byte{1},
	}

	_, err := BuildBlockCandidate(BlockCandidateInput{
		Context: ctx,
		Rules: block.ExecutionRules{
			ChainID:         1002,
			ProtocolVersion: 1,
		},
	}, s)
	if err == nil {
		t.Fatal("expected execution context mismatch")
	}
}

func TestBuildBlockCandidateRejectsNilState(t *testing.T) {
	_, err := BuildBlockCandidate(BlockCandidateInput{}, nil)
	if err != ErrNilBlockProductionState {
		t.Fatalf("expected nil state error, got %v", err)
	}
}

func TestBuildBlockCandidateExecutesSignedTransaction(t *testing.T) {
	signer, err := crypto.GenerateEd25519Signer()
	if err != nil {
		t.Fatalf("generate signer: %v", err)
	}
	sender := signer.PublicKey()
	recipient := types.Address{7, 8, 9}

	s := state.New()
	s.Set(sender, state.Account{Balance: 100, Nonce: 0})

	tx := transaction.Transaction{
		Version:   1,
		ChainID:   1001,
		Nonce:     0,
		Sender:    sender,
		Recipient: recipient,
		Value:     25,
		GasLimit:  1,
	}
	tx.Signature, err = transaction.Sign(tx, signer)
	if err != nil {
		t.Fatalf("sign transaction: %v", err)
	}

	ctx := BlockProductionContext{
		State: RoundState{ProtocolVersion: 1, ChainID: 1001, Height: 3, Phase: PhaseProposal},
		PreviousHash: types.Hash{4},
		Proposer: []byte{1},
	}
	rules := block.ExecutionRules{
		ChainID: 1001,
		ProtocolVersion: 1,
		Transaction: state.ExecutionRules{
			Validation: transaction.ValidationRules{
				ProtocolVersion: 1, ChainID: 1001,
				RequireSender: true, RequireRecipient: true, RequireSignature: true,
				MinGasLimit: 1,
			},
			PublicKey: sender,
		},
	}

	candidate, err := BuildBlockCandidate(BlockCandidateInput{
		Context: ctx, Timestamp: 123, Transactions: []any{tx}, Rules: rules,
	}, s)
	if err != nil {
		t.Fatalf("build candidate: %v", err)
	}

	if got, ok := s.Get(sender); !ok || got.Balance != 100 || got.Nonce != 0 {
		t.Fatalf("canonical sender state mutated: %#v, found=%v", got, ok)
	}
	if got, ok := s.Get(recipient); ok || got.Balance != 0 {
		t.Fatalf("canonical recipient state mutated: %#v, found=%v", got, ok)
	}

	working := s.Snapshot()
	if err := state.ApplyTransaction(working, tx, rules.Transaction); err != nil {
		t.Fatalf("reapply transaction: %v", err)
	}
	if candidate.Header.StateRoot != working.Root() {
		t.Fatalf("candidate state root does not match executed transaction state")
	}
}
