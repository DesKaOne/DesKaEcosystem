package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func mustRuntimeSigner(t *testing.T, seed byte) crypto.Signer {
	t.Helper()
	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{seed}, 32))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func finalizeRuntimeForCanonicalCommitTest(t *testing.T) (*ValidatorRuntime, RoundState, []byte) {
	t.Helper()
	runtime, state, _, _ := runtimeFixture(t)
	payload := make([]byte, len(types.Hash{}))
	payload[0] = 8

	proposal := runtimeMessage(state, "validator-a", MessageTypeProposal, "")
	proposal.Payload = append([]byte(nil), payload...)
	if err := runtime.AcceptProposal(proposal); err != nil {
		t.Fatal(err)
	}
	for _, sender := range []string{"validator-a", "validator-b"} {
		vote := runtimeMessage(state, sender, MessageTypePrevote, "")
		vote.Payload = append([]byte(nil), payload...)
		if err := runtime.AddVote(vote); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		sender string
		seed   byte
	}{
		{"validator-a", 0x31},
		{"validator-b", 0x32},
	} {
		msg := runtimeMessage(tState(runtime), item.sender, MessageTypePrecommit, "")
		msg.Payload = append([]byte(nil), payload...)
		signed, err := msg.Sign(mustRuntimeSigner(t, item.seed))
		if err != nil {
			t.Fatal(err)
		}
		if err := runtime.AddVote(signed); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := runtime.FinalizeProposal(runtimeTestAuthority(t)); err != nil {
		t.Fatal(err)
	}
	return runtime, state, payload
}

func tState(runtime *ValidatorRuntime) RoundState { return runtime.State() }

func TestValidatorRuntimePublishesCanonicalCommitAndAdvancesHeight(t *testing.T) {
	runtime, state, payload := finalizeRuntimeForCanonicalCommitTest(t)
	hash := types.Hash{}
	copy(hash[:], payload)
	commit, err := NewCanonicalCommit(state.Height+1, hash, types.Hash{9})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.PublishCanonicalCommit(commit); err != nil {
		t.Fatal(err)
	}
	got := runtime.State()
	if got.Height != state.Height+1 || got.Round != 0 || got.Phase != PhaseProposal {
		t.Fatalf("unexpected next-height state: height=%d round=%d phase=%v", got.Height, got.Round, got.Phase)
	}
	if runtime.proposal != nil || runtime.lockedProposal != nil || runtime.lockedProof != nil || runtime.certificate != nil {
		t.Fatal("finalized height evidence was not cleared")
	}
	last, err := runtime.LastCanonicalCommit()
	if err != nil {
		t.Fatal(err)
	}
	if last.Height != commit.Height || last.BlockHash != commit.BlockHash || last.StateRoot != commit.StateRoot {
		t.Fatal("canonical commit publication was not retained exactly")
	}
}

func TestValidatorRuntimeRejectsCanonicalCommitBeforeFinalizationWithoutMutation(t *testing.T) {
	runtime, state, _ := runtimeFixture(t)
	payload := make([]byte, len(types.Hash{}))
	payload[0] = 8
	runtime.proposal = append([]byte(nil), payload...)
	hash := types.Hash{}
	copy(hash[:], payload)
	commit, err := NewCanonicalCommit(state.Height+1, hash, types.Hash{9})
	if err != nil {
		t.Fatal(err)
	}
	before := runtime.State()
	if err := runtime.PublishCanonicalCommit(commit); !errors.Is(err, ErrInvalidRuntimePhase) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidRuntimePhase)
	}
	if runtime.State() != before || runtime.lastCanonicalCommit != nil {
		t.Fatal("runtime mutated after pre-finalization rejection")
	}
}

func TestValidatorRuntimeRejectsCanonicalCommitHashMismatchWithoutMutation(t *testing.T) {
	runtime, state, payload := finalizeRuntimeForCanonicalCommitTest(t)
	hash := types.Hash{}
	copy(hash[:], payload)
	hash[0] ^= 0xff
	commit, err := NewCanonicalCommit(state.Height+1, hash, types.Hash{9})
	if err != nil {
		t.Fatal(err)
	}
	before := runtime.State()
	if err := runtime.PublishCanonicalCommit(commit); !errors.Is(err, ErrCanonicalCommitContextMismatch) {
		t.Fatalf("error = %v, want %v", err, ErrCanonicalCommitContextMismatch)
	}
	if runtime.State() != before || runtime.lastCanonicalCommit != nil {
		t.Fatal("runtime mutated after hash-mismatch rejection")
	}
}

func TestValidatorRuntimeRejectsDuplicateCanonicalCommitAfterAdvance(t *testing.T) {
	runtime, state, payload := finalizeRuntimeForCanonicalCommitTest(t)
	hash := types.Hash{}
	copy(hash[:], payload)
	commit, err := NewCanonicalCommit(state.Height+1, hash, types.Hash{9})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.PublishCanonicalCommit(commit); err != nil {
		t.Fatal(err)
	}
	if err := runtime.PublishCanonicalCommit(commit); err == nil {
		t.Fatal("expected duplicate publication rejection")
	}
	if got := runtime.State(); got.Height != state.Height+1 || got.Phase != PhaseProposal {
		t.Fatal("duplicate publication changed next-height state")
	}
}
