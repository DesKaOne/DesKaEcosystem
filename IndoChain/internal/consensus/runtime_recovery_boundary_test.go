package consensus

import (
	"bytes"
	"errors"
	"testing"
)

type recoveryCase struct {
	name string
	mutate func(*RoundState)
	wantErr error
}

func TestConsensusRuntimeRecoveryBoundaryRejectsStaleOrCrossContextState(t *testing.T) {
	base, err := NewRoundState(1, 1001, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	cases := []recoveryCase{
		{name: "protocol-mismatch", mutate: func(s *RoundState) { s.ProtocolVersion++ }, wantErr: ErrInvalidConsensusRuntime},
		{name: "chain-mismatch", mutate: func(s *RoundState) { s.ChainID++ }, wantErr: ErrInvalidConsensusRuntime},
		{name: "height-replay", mutate: func(s *RoundState) { s.Height++ }, wantErr: nil},
		{name: "round-regression", mutate: func(s *RoundState) { s.Round = 0 }, wantErr: nil},
	}
	ids := [][]byte{[]byte("validator-a"), []byte("validator-b")}
	validators, err := NewValidatorSet(ids)
	if err != nil {
		t.Fatal(err)
	}
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: ids[0], Power: 1},
		{ValidatorID: ids[1], Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := base
			tc.mutate(&state)
			_, err := NewValidatorRuntime(RuntimeConfig{
				Rules: ValidationRules{ProtocolVersion: base.ProtocolVersion, ChainID: base.ChainID, RequireSender: true},
				State: state, Validators: validators, VotingPower: power,
				Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
				Proposer: RoundRobinProposer{},
			})
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error = %v", err)
			}
		})
	}
}

func TestConsensusRuntimeRecoveryBoundaryDoesNotPersistEphemeralEvidenceImplicitly(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "recovery-proposal")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-a", MessageTypePrevote, "recovery-proposal")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-b", MessageTypePrevote, "recovery-proposal")); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhasePrecommit {
		t.Fatalf("phase = %v, want precommit", runtime.State().Phase)
	}

	recovered, err := NewValidatorRuntime(RuntimeConfig{
		Rules: runtime.rules, State: runtime.State(), Validators: validators,
		VotingPower: power, Threshold: runtime.threshold, Proposer: runtime.proposer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if recovered.State() != runtime.State() {
		t.Fatalf("recovered state = %+v, want %+v", recovered.State(), runtime.State())
	}
	if len(recovered.Proposal()) != 0 {
		t.Fatalf("proposal unexpectedly restored: %q", recovered.Proposal())
	}
	if len(recovered.PrecommitVotes()) != 0 {
		t.Fatalf("precommit evidence unexpectedly restored: %d", len(recovered.PrecommitVotes()))
	}
	if _, err := recovered.FinalizedCertificate(); !errors.Is(err, ErrInvalidRuntimePhase) {
		t.Fatalf("recovered finality lookup error = %v, want invalid phase", err)
	}
}

func TestConsensusRuntimeRecoveryBoundaryRequiresExplicitEvidenceForFinalityAfterRestore(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "restore-finality")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-a", MessageTypePrevote, "restore-finality")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-b", MessageTypePrevote, "restore-finality")); err != nil {
		t.Fatal(err)
	}

	recovered, err := NewValidatorRuntime(RuntimeConfig{
		Rules: runtime.rules, State: runtime.State(), Validators: validators,
		VotingPower: power, Threshold: runtime.threshold, Proposer: runtime.proposer,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := recovered.State()
	if _, err := recovered.FinalizeProposal(runtimeTestAuthority(t)); err == nil {
		t.Fatal("restored runtime finalized without explicit precommit evidence")
	}
	if recovered.State() != before {
		t.Fatal("failed post-restore finality attempt mutated state")
	}

	// The persistence contract must distinguish protocol state from ephemeral
	// evidence and require explicit authenticated replay of any evidence needed
	// to continue toward finality.
	if !bytes.Equal(recovered.Proposal(), nil) {
		t.Fatal("recovered runtime unexpectedly retained proposal evidence")
	}
}
