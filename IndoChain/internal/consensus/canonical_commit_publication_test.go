package consensus

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func canonicalCommitRuntimeFixture(t *testing.T) (*ValidatorRuntime, CanonicalCommitPublication) {
	t.Helper()
	runtime, state, _, _ := runtimeFixture(t)
	hash := types.Hash{8}
	runtime.state.Phase = PhaseFinalized
	runtime.proposal = append([]byte(nil), hash[:]...)
	publication := CanonicalCommitPublication{
		Height: state.Height + 1,
		BlockHash: hash,
		StateRoot: types.Hash{9},
	}
	return runtime, publication
}

func TestPublishCanonicalCommitAdvancesExactlyOneHeight(t *testing.T) {
	runtime, publication := canonicalCommitRuntimeFixture(t)
	if err := runtime.PublishCanonicalCommit(publication); err != nil {
		t.Fatal(err)
	}
	got := runtime.State()
	if got.Height != publication.Height || got.Round != 0 || got.Phase != PhaseProposal {
		t.Fatalf("unexpected next-height state: height=%d round=%d phase=%v", got.Height, got.Round, got.Phase)
	}
	if runtime.proposal != nil || runtime.lockedProposal != nil || runtime.lockedProof != nil || runtime.certificate != nil {
		t.Fatal("finalized height evidence was not cleared")
	}
}

func TestPublishCanonicalCommitRejectsHashOrHeightMismatchWithoutMutation(t *testing.T) {
	tests := []struct {
		name string
		mutate func(*CanonicalCommitPublication)
	}{
		{"hash", func(p *CanonicalCommitPublication) { p.BlockHash[0] ^= 0xff }},
		{"height", func(p *CanonicalCommitPublication) { p.Height++ }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtime, publication := canonicalCommitRuntimeFixture(t)
			before := runtime.State()
			tc.mutate(&publication)
			err := runtime.PublishCanonicalCommit(publication)
			if !errors.Is(err, ErrCanonicalCommitPublicationContextMismatch) {
				t.Fatalf("error = %v, want %v", err, ErrCanonicalCommitPublicationContextMismatch)
			}
			if runtime.State() != before {
				t.Fatal("runtime advanced after invalid canonical publication")
			}
		})
	}
}

func TestPublishCanonicalCommitRejectsBeforeFinalization(t *testing.T) {
	runtime, publication := canonicalCommitRuntimeFixture(t)
	runtime.state.Phase = PhaseProposal
	before := runtime.State()
	if err := runtime.PublishCanonicalCommit(publication); !errors.Is(err, ErrInvalidRuntimePhase) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRuntimePhase)
	}
	if runtime.State() != before {
		t.Fatal("runtime mutated after pre-finalization rejection")
	}
}
