package consensus

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func roundDriverFixture(t *testing.T) (*RoundDriver, RoundState, StaticValidatorAuthority, map[string]crypto.Signer) {
	t.Helper()
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)
	signers := make(map[string]crypto.Signer)
	for _, item := range []struct {
		id   string
		seed byte
	}{
		{id: "validator-a", seed: 0x31},
		{id: "validator-b", seed: 0x32},
	} {
		kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{item.seed}, ed25519.SeedSize))
		if err != nil {
			t.Fatal(err)
		}
		signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
		signers[item.id] = signer
	}
	driver, err := NewRoundDriver(runtime, authority)
	if err != nil {
		t.Fatal(err)
	}
	return driver, state, authority, signers
}

func TestRoundDriverRejectsUnsignedVoteBeforeAggregation(t *testing.T) {
	driver, state, _, signers := roundDriverFixture(t)
	proposal, err := runtimeMessage(state, "validator-a", MessageTypeProposal, "proposal").Sign(signers["validator-a"])
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.HandleMessage(proposal); err != nil {
		t.Fatal(err)
	}

	unsigned := runtimeMessage(state, "validator-a", MessageTypePrevote, "proposal")
	before := len(driver.runtime.prevotes.Votes)
	if err := driver.HandleMessage(unsigned); !errors.Is(err, ErrMissingSignature) {
		t.Fatalf("unsigned vote error = %v, want %v", err, ErrMissingSignature)
	}
	if len(driver.runtime.prevotes.Votes) != before {
		t.Fatal("unsigned vote reached aggregation")
	}
}

func TestRoundDriverRejectsTamperedVoteBeforeAggregation(t *testing.T) {
	driver, state, _, signers := roundDriverFixture(t)
	proposal, err := runtimeMessage(state, "validator-a", MessageTypeProposal, "proposal").Sign(signers["validator-a"])
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.HandleMessage(proposal); err != nil {
		t.Fatal(err)
	}
	vote, err := runtimeMessage(state, "validator-a", MessageTypePrevote, "proposal").Sign(signers["validator-a"])
	if err != nil {
		t.Fatal(err)
	}
	vote.Signature[0] ^= 0xff
	before := len(driver.runtime.prevotes.Votes)
	if err := driver.HandleMessage(vote); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered vote error = %v, want %v", err, ErrInvalidSignature)
	}
	if len(driver.runtime.prevotes.Votes) != before {
		t.Fatal("tampered vote reached aggregation")
	}
}

func TestRoundDriverQueuesTimeoutAndAdvancesOnlyOnExplicitCommit(t *testing.T) {
	driver, state, _, signers := roundDriverFixture(t)
	timeoutA, err := NewTimeoutMessage(state, []byte("validator-a"), 1, signers["validator-a"])
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := NewTimeoutMessage(state, []byte("validator-b"), 1, signers["validator-b"])
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.HandleMessage(timeoutA); err != nil {
		t.Fatal(err)
	}
	if err := driver.HandleMessage(timeoutB); err != nil {
		t.Fatal(err)
	}
	if driver.Runtime().State().Round != 0 {
		t.Fatalf("round advanced during timeout collection: %d", driver.Runtime().State().Round)
	}
	if len(driver.TimeoutEvidence()) != 2 {
		t.Fatalf("queued timeout evidence = %d, want 2", len(driver.TimeoutEvidence()))
	}
	if _, err := driver.AdvanceRoundFromTimeoutEvidence(); err != nil {
		t.Fatal(err)
	}
	if driver.Runtime().State().Round != 1 {
		t.Fatalf("round after timeout commit = %d, want 1", driver.Runtime().State().Round)
	}
	if len(driver.TimeoutEvidence()) != 0 {
		t.Fatal("timeout evidence was not cleared after successful transition")
	}
}

func TestRoundDriverKeepsFailedTimeoutBatchForRecovery(t *testing.T) {
	driver, state, _, signers := roundDriverFixture(t)
	timeoutA, err := NewTimeoutMessageWithLock(state, []byte("validator-a"), 1, []byte("lock-a"), signers["validator-a"])
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := NewTimeoutMessageWithLock(state, []byte("validator-b"), 1, []byte("lock-b"), signers["validator-b"])
	if err != nil {
		t.Fatal(err)
	}
	if err := driver.HandleMessage(timeoutA); err != nil {
		t.Fatal(err)
	}
	if err := driver.HandleMessage(timeoutB); err != nil {
		t.Fatal(err)
	}
	before := driver.Runtime().State()
	if _, err := driver.AdvanceRoundFromTimeoutEvidence(); !errors.Is(err, ErrConflictingTimeoutLock) {
		t.Fatalf("conflicting timeout error = %v, want %v", err, ErrConflictingTimeoutLock)
	}
	if driver.Runtime().State() != before {
		t.Fatal("failed timeout batch mutated runtime")
	}
	if len(driver.TimeoutEvidence()) != 2 {
		t.Fatal("failed timeout batch was silently discarded")
	}
}
