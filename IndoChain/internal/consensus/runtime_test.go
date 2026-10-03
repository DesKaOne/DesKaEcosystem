package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func runtimeFixture(t *testing.T) (*ValidatorRuntime, RoundState, ValidatorSet, VotingPowerSet) {
	t.Helper()
	state, err := NewRoundState(1, 1001, 1, 8)
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b"), []byte("validator-c")})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 4},
		{ValidatorID: []byte("validator-b"), Power: 3},
		{ValidatorID: []byte("validator-c"), Power: 3},
	})
	if err != nil { t.Fatal(err) }

	runtime, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID: state.ChainID,
			RequireSender: true,
		},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	return runtime, state, validators, power
}

func runtimeTestAuthoritySet(t *testing.T, state RoundState, validators ValidatorSet) ValidatorAuthoritySet {
	t.Helper()
	base := runtimeTestAuthority(t)
	keys := map[string][]byte{}
	for _, id := range validators.Validators {
		key, err := base.PublicKeyForValidator(id)
		if err != nil { t.Fatal(err) }
		keys[string(id)] = key
	}
	set, err := NewValidatorAuthoritySet(state.Epoch, validators, keys)
	if err != nil { t.Fatal(err) }
	return set
}

func runtimeTestAuthority(t *testing.T) StaticValidatorAuthority {
	t.Helper()
	keys := map[string][]byte{}
	for _, item := range []struct{ id string; seed byte }{{"validator-a", 0x31}, {"validator-b", 0x32}} {
		kp, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{item.seed}, 32))
		if err != nil { t.Fatal(err) }
		keys[item.id] = append([]byte(nil), kp.PublicKey...)
	}
	authority, err := NewStaticValidatorAuthority(keys)
	if err != nil { t.Fatal(err) }
	return authority
}

func runtimeSignedMessage(t *testing.T, state RoundState, sender string, kind MessageType, payload string) Message {
	t.Helper()
	var seed byte
	switch sender {
	case "validator-a": seed = 0x31
	case "validator-b": seed = 0x32
	default: t.Fatalf("unknown runtime test validator %q", sender)
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

func runtimeMessage(state RoundState, sender string, kind MessageType, payload string) Message {
	return Message{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
		Epoch: state.Epoch, Height: state.Height, Round: state.Round,
		Sender: []byte(sender), Type: kind, Payload: []byte(payload),
	}
}

func TestValidatorRuntimeAcceptsExpectedProposer(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrevote {
		t.Fatalf("expected prevote phase, got %v", runtime.State().Phase)
	}
}

func TestValidatorRuntimeRejectsUnexpectedProposer(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	err := runtime.AcceptProposal(runtimeMessage(state, "validator-b", MessageTypeProposal, "block-8"))
	if !errors.Is(err, ErrUnexpectedProposer) {
		t.Fatalf("expected unexpected proposer error, got %v", err)
	}
	if runtime.State().Phase != PhaseProposal {
		t.Fatalf("runtime phase changed after rejected proposal")
	}
}

func TestValidatorRuntimeAdvancesAndFinalizesAfterQuorum(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrevote {
		t.Fatalf("expected prevote before quorum, got %v", runtime.State().Phase)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrecommit {
		t.Fatalf("expected precommit after prevote quorum, got %v", runtime.State().Phase)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-a", MessageTypePrecommit, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-b", MessageTypePrecommit, "block-8")); err != nil {
		t.Fatal(err)
	}

	certificate, err := runtime.FinalizeProposal(runtimeTestAuthority(t))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhaseFinalized {
		t.Fatalf("expected finalized phase, got %v", runtime.State().Phase)
	}
	if string(certificate.Payload) != "block-8" {
		t.Fatalf("unexpected certificate payload %q", certificate.Payload)
	}
}

func TestValidatorRuntimeRejectsVoteConflictingWithLockedProposal(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrecommit {
		t.Fatalf("expected precommit after locking proposal, got %v", runtime.State().Phase)
	}

	err := runtime.AddVote(runtimeMessage(state, "validator-c", MessageTypePrecommit, "conflicting-block"))
	if !errors.Is(err, ErrConflictingLockedProposal) {
		t.Fatalf("expected locked-proposal conflict, got %v", err)
	}
	if runtime.State().Phase != PhasePrecommit {
		t.Fatalf("runtime phase changed after conflicting locked vote")
	}
	if votes := runtime.prevotes.VotesForPayload([]byte("conflicting-block")); len(votes) != 0 {
		t.Fatalf("conflicting vote was recorded: %d", len(votes))
	}
}

func TestValidatorRuntimeAdvancesRoundPreservingLock(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AdvanceRound(1); err != nil {
		t.Fatal(err)
	}
	if got := runtime.State(); got.Round != 1 || got.Phase != PhaseProposal {
		t.Fatalf("unexpected round state after round change: round=%d phase=%v", got.Round, got.Phase)
	}
	if runtime.proposal != nil || len(runtime.prevotes.Votes) != 0 {
		t.Fatal("round-local proposal or votes were not reset")
	}

	newRoundState := runtime.State()
	if err := runtime.AcceptProposal(runtimeMessage(newRoundState, "validator-b", MessageTypeProposal, "block-8")); err != nil {
		t.Fatalf("locked proposal should remain acceptable in the next round: %v", err)
	}
}

func TestValidatorRuntimeRejectsConflictingProposalAfterRoundChange(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AdvanceRound(1); err != nil {
		t.Fatal(err)
	}

	newRoundState := runtime.State()
	err := runtime.AcceptProposal(runtimeMessage(newRoundState, "validator-b", MessageTypeProposal, "block-9"))
	if !errors.Is(err, ErrConflictingLockedProposal) {
		t.Fatalf("expected locked proposal conflict, got %v", err)
	}
	if got := runtime.State(); got.Round != 1 || got.Phase != PhaseProposal {
		t.Fatalf("runtime changed after conflicting proposal: round=%d phase=%v", got.Round, got.Phase)
	}
	if runtime.proposal != nil || len(runtime.prevotes.Votes) != 0 {
		t.Fatal("conflicting proposal mutated round-local state")
	}
}

func TestValidatorRuntimeRejectsRoundChangeAfterFinalization(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-a", MessageTypePrecommit, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-b", MessageTypePrecommit, "block-8")); err != nil { t.Fatal(err) }
	if _, err := runtime.FinalizeProposal(runtimeTestAuthority(t)); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AdvanceRound(1); !errors.Is(err, ErrRoundChangeFinalized) {
		t.Fatalf("expected finalized round-change rejection, got %v", err)
	}
	if got := runtime.State(); got.Round != 0 || got.Phase != PhaseFinalized {
		t.Fatalf("runtime changed after rejected finalized round change: round=%d phase=%v", got.Round, got.Phase)
	}
}

func TestValidatorRuntimeDoesNotFinalizeWithoutQuorum(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil {
		t.Fatal(err)
	}
	_, err := runtime.FinalizeProposal(runtimeTestAuthority(t))
	if !errors.Is(err, ErrInvalidRuntimePhase) {
		t.Fatalf("expected invalid runtime phase, got %v", err)
	}
}

func TestValidatorRuntimeAcceptsValidatedBlockProposal(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	ctx := BlockProductionContext{
		State: state,
		PreviousHash: types.Hash{5},
		Proposer: []byte("validator-a"),
	}
	candidate := block.Block{Header: block.Header{
		Version: state.ProtocolVersion,
		ChainID: state.ChainID,
		Height: state.Height + 1,
		PreviousHash: ctx.PreviousHash,
		Proposer: append([]byte(nil), ctx.Proposer...),
	}}
	proposal, err := NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatalf("NewBlockProposal() error = %v", err)
	}
	if err := runtime.AcceptBlockProposal(proposal); err != nil {
		t.Fatalf("AcceptBlockProposal() error = %v", err)
	}
	if runtime.State().Phase != PhasePrevote {
		t.Fatalf("expected prevote phase, got %v", runtime.State().Phase)
	}
}

func TestValidatorRuntimeRejectsBlockProposalFromWrongContext(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	ctx := BlockProductionContext{
		State: state,
		PreviousHash: types.Hash{5},
		Proposer: []byte("validator-b"),
	}
	candidate := block.Block{Header: block.Header{
		Version: state.ProtocolVersion,
		ChainID: state.ChainID,
		Height: state.Height + 1,
		PreviousHash: ctx.PreviousHash,
		Proposer: append([]byte(nil), ctx.Proposer...),
	}}
	proposal, err := NewBlockProposal(ctx, candidate)
	if err != nil {
		t.Fatalf("NewBlockProposal() error = %v", err)
	}
	if err := runtime.AcceptBlockProposal(proposal); !errors.Is(err, ErrUnexpectedProposer) {
		t.Fatalf("expected unexpected proposer, got %v", err)
	}
	if runtime.State().Phase != PhaseProposal {
		t.Fatalf("runtime phase changed after rejected block proposal")
	}
}

func TestValidatorRuntimeExposesClonedFinalityCertificate(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-a", MessageTypePrecommit, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-b", MessageTypePrecommit, "block-8")); err != nil { t.Fatal(err) }
	if _, err := runtime.FinalizeProposal(runtimeTestAuthority(t)); err != nil { t.Fatal(err) }
	certificate, err := runtime.FinalizedCertificate(); if err != nil { t.Fatal(err) }
	certificate.Payload[0] = 'X'
	certificate.Votes[0].Payload[0] = 'Y'
	fresh, err := runtime.FinalizedCertificate(); if err != nil { t.Fatal(err) }
	if string(fresh.Payload) != "block-8" || string(fresh.Votes[0].Payload) != "block-8" { t.Fatal("runtime certificate was not cloned") }
}

func TestValidatorRuntimeRejectsAuthorityEpochMismatch(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	bad, err := NewValidatorAuthoritySet(state.Epoch+1, validators, map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	})
	if err != nil { t.Fatal(err) }
	_, err = NewValidatorRuntime(RuntimeConfig{
		Authority: &bad,
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3}, Proposer: RoundRobinProposer{},
	})
	if !errors.Is(err, ErrValidatorAuthorityMismatch) {
		t.Fatalf("expected authority epoch mismatch, got %v", err)
	}
}

func TestValidatorRuntimeAuthoritySnapshotIsDefensive(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	keys := map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	}
	authoritySnapshot, err := NewValidatorAuthoritySet(state.Epoch, validators, keys)
	if err != nil { t.Fatal(err) }
	runtime, err := NewValidatorRuntime(RuntimeConfig{
		Authority: &authoritySnapshot,
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3}, Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	authority, err := runtime.Authority()
	if err != nil { t.Fatal(err) }
	key, err := authority.PublicKeyForValidator([]byte("validator-a"))
	if err != nil { t.Fatal(err) }
	key[0] = 'X'
	fresh, err := runtime.Authority()
	if err != nil { t.Fatal(err) }
	freshKey, err := fresh.PublicKeyForValidator([]byte("validator-a"))
	if err != nil { t.Fatal(err) }
	if bytes.Equal(freshKey, key) { t.Fatal("runtime authority leaked mutable key alias") }
}


func TestValidatorRuntimeBuildsAuthorityBoundPersistenceContext(t *testing.T) {
	runtime, _, _, _ := runtimeFixture(t)
	if _, err := runtime.ConsensusAuthority(); err == nil {
		t.Fatal("legacy runtime unexpectedly exposes authority")
	}

	_, state, validators, power := runtimeFixture(t)
	keys := map[string][]byte{
		"validator-a": []byte("key-a"),
		"validator-b": []byte("key-b"),
		"validator-c": []byte("key-c"),
	}
	authority, err := NewValidatorAuthoritySet(state.Epoch, validators, keys)
	if err != nil { t.Fatal(err) }
	runtime, err = NewValidatorRuntime(RuntimeConfig{
		Authority: &authority,
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3}, Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }

	context, err := runtime.PersistenceContext([32]byte{7}, [32]byte{8}, "round-robin-v0-dev", "1")
	if err != nil { t.Fatal(err) }
	if context.Epoch != state.Epoch || context.Height != uint64(state.Height) || context.Round != state.Round || context.Phase != uint8(state.Phase) {
		t.Fatalf("persistence context state mismatch: %+v", context)
	}
	wantAuthorityDigest := authorityDigest(authority)
	if context.ValidatorAuthorityDigest != wantAuthorityDigest {
		t.Fatal("persistence context did not use runtime authority digest")
	}
	if context.ThresholdNumerator != 2 || context.ThresholdDenominator != 3 {
		t.Fatal("persistence context threshold mismatch")
	}

	contextAgain, err := runtime.PersistenceContext([32]byte{7}, [32]byte{8}, "round-robin-v0-dev", "1")
	if err != nil { t.Fatal(err) }
	if PersistenceContextDigest(context) != PersistenceContextDigest(contextAgain) {
		t.Fatal("runtime-derived persistence context is not deterministic")
	}
}

func TestValidatorRuntimePersistenceContextRejectsMissingAuthority(t *testing.T) {
	runtime, _, _, _ := runtimeFixture(t)
	if _, err := runtime.PersistenceContext([32]byte{1}, [32]byte{2}, "round-robin-v0-dev", "1"); err != ErrAuthenticatedConsensusAuthorityMissing {
		t.Fatalf("error = %v, want %v", err, ErrAuthenticatedConsensusAuthorityMissing)
	}
}
