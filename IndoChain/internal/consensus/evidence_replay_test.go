package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func replaySignedMessage(t *testing.T, state RoundState, sender string, kind MessageType, payload string) Message {
	t.Helper()
	var seed byte
	switch sender {
	case "validator-a":
		seed = 0x31
	case "validator-b":
		seed = 0x32
	default:
		t.Fatalf("unknown sender %q", sender)
	}
	kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{seed}, 32))
	if err != nil { t.Fatal(err) }
	signer, err := crypto.NewEd25519Signer(kp.PrivateKey)
	if err != nil { t.Fatal(err) }
	msg := runtimeMessage(state, sender, kind, payload)
	signed, err := msg.Sign(signer)
	if err != nil { t.Fatal(err) }
	return signed
}

func TestReplayAuthenticatedEvidenceRebuildsRoundState(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)

	proposal := replaySignedMessage(t, state, "validator-a", MessageTypeProposal, "block-8")
	prevoteA := replaySignedMessage(t, state, "validator-a", MessageTypePrevote, "block-8")
	prevoteB := replaySignedMessage(t, state, "validator-b", MessageTypePrevote, "block-8")
	precommitAState := state
	precommitAState.Phase = PhasePrecommit
	precommitA := replaySignedMessage(t, precommitAState, "validator-a", MessageTypePrecommit, "block-8")
	precommitB := replaySignedMessage(t, precommitAState, "validator-b", MessageTypePrecommit, "block-8")

	result, err := ReplayAuthenticatedEvidence(runtime, []Message{
		precommitB, prevoteB, proposal, precommitA, prevoteA,
	}, authority)
	if err != nil { t.Fatal(err) }
	if result.AppliedProposals != 1 || result.AppliedVotes != 4 || result.ValidatedFinality != 0 {
		t.Fatalf("unexpected replay result: %+v", result)
	}
	if got := runtime.State(); got.Phase != PhasePrecommit || got.Round != 0 {
		t.Fatalf("unexpected recovered runtime state: %+v", got)
	}
}

func TestReplayAuthenticatedEvidenceRejectsRoundGapWithoutMutation(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)
	state.Round = 1
	proposal := replaySignedMessage(t, state, "validator-a", MessageTypeProposal, "block-8")

	_, err := ReplayAuthenticatedEvidence(runtime, []Message{proposal}, authority)
	if !errors.Is(err, ErrReplayRoundGap) {
		t.Fatalf("expected round-gap rejection, got %v", err)
	}
	if got := runtime.State(); got.Round != 0 || got.Phase != PhaseProposal {
		t.Fatalf("runtime mutated after round-gap rejection: %+v", got)
	}
}

func TestReplayAuthenticatedEvidenceDoesNotImplicitlyFinalize(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	authority := runtimeTestAuthority(t)

	proposal := replaySignedMessage(t, state, "validator-a", MessageTypeProposal, "block-8")
	prevoteA := replaySignedMessage(t, state, "validator-a", MessageTypePrevote, "block-8")
	prevoteB := replaySignedMessage(t, state, "validator-b", MessageTypePrevote, "block-8")
	precommitState := state
	precommitState.Phase = PhasePrecommit
	precommitA := replaySignedMessage(t, precommitState, "validator-a", MessageTypePrecommit, "block-8")
	precommitB := replaySignedMessage(t, precommitState, "validator-b", MessageTypePrecommit, "block-8")

	result, err := ReplayAuthenticatedEvidence(runtime, []Message{
		proposal, prevoteA, prevoteB, precommitA, precommitB,
	}, authority)
	if err != nil { t.Fatal(err) }
	if result.ValidatedFinality != 0 || runtime.State().Phase != PhasePrecommit {
		t.Fatalf("replay implicitly finalized runtime: result=%+v state=%+v", result, runtime.State())
	}
}
