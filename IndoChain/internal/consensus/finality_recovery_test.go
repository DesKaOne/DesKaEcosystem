package consensus

import "testing"

func TestValidatorRuntimeRestoresFinalityEvidenceAfterRestart(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-a", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeMessage(state, "validator-b", MessageTypePrevote, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-a", MessageTypePrecommit, "block-8")); err != nil { t.Fatal(err) }
	if err := runtime.AddVote(runtimeSignedMessage(t, runtime.State(), "validator-b", MessageTypePrecommit, "block-8")); err != nil { t.Fatal(err) }
	certificate, err := runtime.FinalizeProposal(runtimeTestAuthority(t))
	if err != nil { t.Fatal(err) }

	restarted, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3}, Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	if err := restarted.RestoreFinalizedEvidence(certificate, runtimeTestAuthority(t)); err != nil {
		t.Fatal(err)
	}
	if got := restarted.State(); got.Phase != PhaseFinalized || got.Round != certificate.Round {
		t.Fatalf("unexpected restored runtime: %+v", got)
	}
	if got := restarted.Proposal(); string(got) != "block-8" {
		t.Fatalf("restored proposal = %q", got)
	}
	got, err := restarted.FinalizedCertificate()
	if err != nil { t.Fatal(err) }
	if string(got.Payload) != "block-8" || len(got.Votes) != len(certificate.Votes) {
		t.Fatal("restored certificate mismatch")
	}
}

func TestValidatorRuntimeRejectsConflictingFinalityRestore(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	certificate := FinalityCertificate{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: state.Round,
		Payload: []byte("block-8"),
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
	}
	if err := runtime.RestoreFinalizedEvidence(certificate, runtimeTestAuthority(t)); err == nil {
		t.Fatal("expected incomplete certificate rejection")
	}
	if got := runtime.State(); got.Phase != PhaseProposal {
		t.Fatalf("runtime changed after rejected restore: %+v", got)
	}
	_ = validators
	_ = power
}
