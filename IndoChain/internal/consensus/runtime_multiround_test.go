package consensus

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"
)

func newTimeoutTestSignerPair(t *testing.T) (timeoutTestSigner, timeoutTestSigner, timeoutRuntimeAuthorityResolver) {
	t.Helper()
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	return signerA, signerB, timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
}

func timeoutMessagesForProof(t *testing.T, state RoundState, proof LockProof, signerA, signerB timeoutTestSigner) []Message {
	t.Helper()
	msgA, err := NewTimeoutMessageWithLockProof(state, []byte("validator-a"), state.Round+1, proof, signerA)
	if err != nil { t.Fatal(err) }
	msgB, err := NewTimeoutMessageWithLockProof(state, []byte("validator-b"), state.Round+1, proof, signerB)
	if err != nil { t.Fatal(err) }
	return []Message{msgA, msgB}
}

func primeRuntimeLock(t *testing.T, runtime *ValidatorRuntime, state RoundState, proposal string) {
	t.Helper()
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, proposal)); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, proposal)); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, proposal)); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrecommit {
		t.Fatalf("runtime did not enter precommit phase: %v", runtime.State().Phase)
	}
}

func TestValidatorRuntimeRejectsLowerTimeoutLockWithoutMutation(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	runtime.state.Round = 2
	runtime.lockedProposal = []byte("locked-proposal")
	runtime.lockedRound = 1

	signerA, signerB, resolver := newTimeoutTestSignerPair(t)


	proof := timeoutLockProofAtRound(t, state, validators, power, 0, "locked-proposal", signerA, signerB)


	messages := timeoutMessagesForProof(t, runtime.State(), proof, signerA, signerB)

	beforeState := runtime.state
	beforeProposal := append([]byte(nil), runtime.lockedProposal...)
	beforeRound := runtime.lockedRound
	_, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver)
	if !errors.Is(err, ErrStaleTimeoutLock) {
		t.Fatalf("expected stale timeout lock rejection, got %v", err)
	}
	if runtime.state != beforeState || !bytes.Equal(runtime.lockedProposal, beforeProposal) || runtime.lockedRound != beforeRound {
		t.Fatal("runtime state or lock changed after stale timeout lock rejection")
	}
}

func TestValidatorRuntimeAdoptsHigherLockWithDifferentProposal(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	runtime.state.Round = 1
	runtime.lockedProposal = []byte("old-proposal")
	runtime.lockedRound = 0

	signerA, signerB, resolver := newTimeoutTestSignerPair(t)


	proof := timeoutLockProofAtRound(t, state, validators, power, 1, "new-proposal", signerA, signerB)


	messages := timeoutMessagesForProof(t, runtime.State(), proof, signerA, signerB)

	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver); err != nil {
		t.Fatalf("higher-lock adoption failed: %v", err)
	}
	if runtime.State().Round != 2 || runtime.State().Phase != PhaseProposal {
		t.Fatalf("unexpected round state: round=%d phase=%v", runtime.State().Round, runtime.State().Phase)
	}
	if !bytes.Equal(runtime.lockedProposal, []byte("new-proposal")) || runtime.lockedRound != 1 {
		t.Fatalf("higher lock was not adopted: proposal=%q round=%d", runtime.lockedProposal, runtime.lockedRound)
	}
	if runtime.lockedProof == nil || !bytes.Equal(runtime.lockedProof.Proposal, []byte("new-proposal")) {
		t.Fatal("authenticated higher-lock proof was not retained")
	}
}

func TestValidatorRuntimeRejectsWrongContextTimeoutLockProofs(t *testing.T) {
	tests := []struct {
		name string
		mutate func(*RoundState)
	}{
		{name: "height", mutate: func(s *RoundState) { s.Height++ }},
		{name: "chain", mutate: func(s *RoundState) { s.ChainID++ }},
		{name: "epoch", mutate: func(s *RoundState) { s.Epoch++ }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runtime, state, validators, power := runtimeFixture(t)
			proofState := state
			tc.mutate(&proofState)
			signerA, signerB, resolver := newTimeoutTestSignerPair(t)

			proof := timeoutLockProofAtRound(t, proofState, validators, power, proofState.Round, "context-proposal", signerA, signerB)

			messages := timeoutMessagesForProof(t, runtime.State(), proof, signerA, signerB)

			before := runtime.state
			_, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver)
			if !errors.Is(err, ErrStateContextMismatch) {
				t.Fatalf("expected context mismatch, got %v", err)
			}
			if runtime.state != before {
				t.Fatal("runtime round state changed after wrong-context lock proof")
			}
		})
	}
}

func TestValidatorRuntimeRejectsTamperedPrecommitSignatureInTimeoutLockProof(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	signerA, signerB, resolver := newTimeoutTestSignerPair(t)

	proof := timeoutLockProofAtRound(t, state, validators, power, state.Round, "tampered-proposal", signerA, signerB)

	proof.Certificate.Votes[0].Signature[0] ^= 0xff

	messages := timeoutMessagesForProof(t, runtime.State(), proof, signerA, signerB)

	beforeState := runtime.state
	beforeProposal := append([]byte(nil), runtime.lockedProposal...)
	beforeRound := runtime.lockedRound
	_, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
	if runtime.state != beforeState || !bytes.Equal(runtime.lockedProposal, beforeProposal) || runtime.lockedRound != beforeRound {
		t.Fatal("runtime mutated after tampered authenticated lock proof")
	}
}

func TestValidatorRuntimeRejectsReplayedTimeoutEvidenceAfterRoundChange(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
	}}
	msgA, err := NewTimeoutMessage(state, []byte("validator-a"), state.Round+1, signerA)
	if err != nil { t.Fatal(err) }
	msgB, err := NewTimeoutMessage(state, []byte("validator-b"), state.Round+1, signerB)
	if err != nil { t.Fatal(err) }
	messages := []Message{msgA, msgB}
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver); err != nil {
		t.Fatal(err)
	}
	before := runtime.state
	_, err = runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver)
	if !errors.Is(err, ErrStateContextMismatch) {
		t.Fatalf("expected replay/context rejection, got %v", err)
	}
	if runtime.state != before {
		t.Fatal("runtime round changed after replayed timeout evidence")
	}
}

func TestValidatorRuntimeHigherLockValidationFailureIsAtomic(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	runtime.state.Round = 1
	runtime.lockedProposal = []byte("old-proposal")
	runtime.lockedRound = 0
	signerA, signerB, resolver := newTimeoutTestSignerPair(t)
	proof := timeoutLockProofAtRound(t, state, validators, power, 1, "new-proposal", signerA, signerB)
	proof.Certificate.Votes[0].Signature[0] ^= 0xff
	messages := timeoutMessagesForProof(t, runtime.State(), proof, signerA, signerB)

	beforeState := runtime.state
	beforeProposal := append([]byte(nil), runtime.lockedProposal...)
	beforeRound := runtime.lockedRound
	beforeProof := cloneLockProofPtr(runtime.lockedProof)
	_, err := runtime.AdvanceRoundWithTimeoutEvidence(messages, resolver)
	if !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
	if runtime.state != beforeState || !bytes.Equal(runtime.lockedProposal, beforeProposal) || runtime.lockedRound != beforeRound {
		t.Fatal("runtime changed after failed higher-lock validation")
	}
	if !lockProofEqual(runtime.lockedProof, beforeProof) {
		t.Fatal("runtime lock proof changed after failed higher-lock validation")
	}
}

func TestValidatorRuntimeMultiRoundLockTimeoutAndAuthenticatedFinality(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	primeRuntimeLock(t, runtime, state, "round-0-proposal")

	// Round 0 -> Round 1: carry the authenticated round-0 lock.
	signer0A, signer0B, resolver0 := newTimeoutTestSignerPair(t)

	proof0 := timeoutLockProofAtRound(t, state, validators, power, 0, "round-0-proposal", signer0A, signer0B)

	messages0 := timeoutMessagesForProof(t, runtime.State(), proof0, signer0A, signer0B)
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(messages0, resolver0); err != nil {
		t.Fatalf("round-0 timeout transition failed: %v", err)
	}

	// Round 1 -> Round 2: adopt a higher authenticated lock for a new proposal.
	state1 := runtime.State()
	signer1A, signer1B, resolver1 := newTimeoutTestSignerPair(t)

	proof1 := timeoutLockProofAtRound(t, state1, validators, power, 1, "round-1-proposal", signer1A, signer1B)

	messages1 := timeoutMessagesForProof(t, runtime.State(), proof1, signer1A, signer1B)
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(messages1, resolver1); err != nil {
		t.Fatalf("higher-lock round-1 adoption failed: %v", err)
	}
	if runtime.lockedRound != 1 || !bytes.Equal(runtime.lockedProposal, []byte("round-1-proposal")) {
		t.Fatal("round-1 higher lock was not adopted")
	}

	// Round 2 -> Round 3: preserve the adopted lock through another timeout.
	state2 := runtime.State()
	messages2 := timeoutMessagesForProof(t, state2, proof1, signer1A, signer1B)
	resolver2 := resolver1
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(messages2, resolver2); err != nil {
		t.Fatalf("round-2 timeout transition failed: %v", err)
	}
	if runtime.State().Round != 3 || runtime.State().Phase != PhaseProposal {
		t.Fatalf("unexpected round-3 state: round=%d phase=%v", runtime.State().Round, runtime.State().Phase)
	}

	// Round 3 finalization must use explicit authenticated precommit evidence.
	state3 := runtime.State()
	if err := runtime.AcceptProposal(runtimeMessage(state3, "validator-a", MessageTypeProposal, "round-1-proposal")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state3, "validator-a", MessageTypePrevote, "round-1-proposal")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state3, "validator-b", MessageTypePrevote, "round-1-proposal")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-a", MessageTypePrecommit, "round-1-proposal")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-b", MessageTypePrecommit, "round-1-proposal")); err != nil {
		t.Fatal(err)
	}
	certificate, err := runtime.FinalizeProposal(runtimeTestAuthority(t))
	if err != nil { t.Fatalf("round-3 authenticated finality failed: %v", err) }
	if runtime.State().Phase != PhaseFinalized {
		t.Fatalf("runtime did not finalize at round 3: %v", runtime.State().Phase)
	}
	for _, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit || len(vote.Signature) == 0 {
			t.Fatal("finality certificate contains non-authenticated/non-precommit evidence")
		}
	}
}

func lockProofEqual(a, b *LockProof) bool {
	if a == nil || b == nil { return a == b }
	if a.LockedRound != b.LockedRound || !bytes.Equal(a.Proposal, b.Proposal) {
		return false
	}
	if a.Certificate.ProtocolVersion != b.Certificate.ProtocolVersion ||
		a.Certificate.ChainID != b.Certificate.ChainID ||
		a.Certificate.Epoch != b.Certificate.Epoch ||
		a.Certificate.Height != b.Certificate.Height ||
		a.Certificate.Round != b.Certificate.Round ||
		a.Certificate.Threshold != b.Certificate.Threshold ||
		!bytes.Equal(a.Certificate.Payload, b.Certificate.Payload) ||
		!sameConsensusEvidence(a.Certificate.Votes, b.Certificate.Votes) {
		return false
	}
	return true
}
