package consensus

import (
	"bytes"
	"crypto/ed25519"
	"testing"
)

type simulatedValidator struct {
	id     []byte
	signer timeoutTestSigner
	public  ed25519.PublicKey
}

func newSimulatedRuntimeCluster(t *testing.T, state RoundState) ([]*ValidatorRuntime, []simulatedValidator, ValidatorSet, VotingPowerSet) {
	t.Helper()
	ids := [][]byte{[]byte("validator-a"), []byte("validator-b"), []byte("validator-c")}
	validators, err := NewValidatorSet(ids)
	if err != nil {
		t.Fatal(err)
	}
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: ids[0], Power: 1},
		{ValidatorID: ids[1], Power: 1},
		{ValidatorID: ids[2], Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	simulated := make([]simulatedValidator, 0, len(ids))
	for _, id := range ids {
		signer, public := newTimeoutTestSigner(t)
		simulated = append(simulated, simulatedValidator{id: append([]byte(nil), id...), signer: signer, public: public})
	}
	rules := ValidationRules{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		RequireSender:   true,
	}
	runtimes := make([]*ValidatorRuntime, 0, len(ids))
	for range ids {
		runtime, err := NewValidatorRuntime(RuntimeConfig{
			Rules: rules, State: state, Validators: validators, VotingPower: power,
			Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
			Proposer:  RoundRobinProposer{},
		})
		if err != nil {
			t.Fatal(err)
		}
		runtimes = append(runtimes, runtime)
	}
	return runtimes, simulated, validators, power
}

func signedSimulationMessage(t *testing.T, state RoundState, validator simulatedValidator, typ MessageType, payload string) Message {
	t.Helper()
	msg := Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Sender:          append([]byte(nil), validator.id...),
		Type:            typ,
		Payload:         []byte(payload),
	}
	signed, err := msg.Sign(validator.signer)
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func TestDeterministicRoundDriverSimulationThreeNodes(t *testing.T) {
	state, err := NewRoundState(1, 1001, 2, 9)
	if err != nil {
		t.Fatal(err)
	}
	runtimes, validatorsFixture, validators, _ := newSimulatedRuntimeCluster(t, state)
	proposer, err := runtimes[0].ExpectedProposer()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(proposer, validatorsFixture[0].id) {
		t.Fatalf("round-0 proposer = %q, want %q", proposer, validatorsFixture[0].id)
	}

	proposal := signedSimulationMessage(t, state, validatorsFixture[0], MessageTypeProposal, "round-0-proposal")
	for i, runtime := range runtimes {
		if err := runtime.AcceptProposal(proposal); err != nil {
			t.Fatalf("node %d rejected proposal: %v", i, err)
		}
	}

	for _, validator := range validatorsFixture[:2] {
		vote := signedSimulationMessage(t, state, validator, MessageTypePrevote, "round-0-proposal")
		for i, runtime := range runtimes {
			if err := runtime.AddVote(vote); err != nil {
				t.Fatalf("node %d rejected prevote from %q: %v", i, validator.id, err)
			}
		}
	}
	for i, runtime := range runtimes {
		if runtime.State().Phase != PhasePrecommit {
			t.Fatalf("node %d phase = %v, want precommit", i, runtime.State().Phase)
		}
	}

	signerA, publicA := newTimeoutTestSigner(t)
	signerB, publicB := newTimeoutTestSigner(t)
	_ = signerA
	_ = signerB
	resolver := timeoutRuntimeAuthorityResolver{keys: map[string]ed25519.PublicKey{
		"validator-a": publicA,
		"validator-b": publicB,
		"validator-c": validatorsFixture[2].public,
	}}
	timeoutA, err := NewTimeoutMessage(runtimes[0].State(), validatorsFixture[0].id, 1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	timeoutB, err := NewTimeoutMessage(runtimes[0].State(), validatorsFixture[1].id, 1, signerB)
	if err != nil {
		t.Fatal(err)
	}

	for i, runtime := range runtimes {
		if _, err := runtime.AdvanceRoundWithTimeoutEvidence([]Message{timeoutB, timeoutA}, resolver); err != nil {
			t.Fatalf("node %d failed deterministic timeout transition: %v", i, err)
		}
		if runtime.State().Round != 1 || runtime.State().Phase != PhaseProposal {
			t.Fatalf("node %d state after timeout = %+v", i, runtime.State())
		}
		if len(runtime.PrecommitVotes()) != 0 {
			t.Fatalf("node %d retained round-local precommit evidence after round change", i)
		}
		if !bytes.Equal(runtime.Validators().Validators[0], validators.Validators[0]) {
			t.Fatalf("node %d validator set changed across round transition", i)
		}
	}

	for i, runtime := range runtimes {
		if err := runtime.AcceptProposal(signedSimulationMessage(t, runtime.State(), validatorsFixture[2], MessageTypeProposal, "round-1-proposal")); err == nil {
			t.Fatalf("node %d accepted non-proposer round-1 proposal", i)
		}
	}
}

func TestDeterministicRoundDriverRecoveryBoundaryDoesNotInventPersistedLock(t *testing.T) {
	runtime, state, validators, power := runtimeFixture(t)
	primeRuntimeLock(t, runtime, state, "persisted-lock-candidate")

	signerA, signerB, resolver := newTimeoutTestSignerPair(t)
	proof := timeoutLockProofAtRound(t, state, validators, power, 0, "persisted-lock-candidate", signerA, signerB)
	if _, err := runtime.AdvanceRoundWithTimeoutEvidence(timeoutMessagesForProof(t, runtime.State(), proof, signerA, signerB), resolver); err != nil {
		t.Fatal(err)
	}
	if runtime.State().Round != 1 || runtime.lockedProof == nil {
		t.Fatal("expected authenticated lock to exist before simulated restart")
	}

	// Simulated restart deliberately reconstructs only the durable state boundary
	// that currently exists: protocol/chain/epoch/height/round/phase. Consensus
	// lock/certificate persistence is not implemented yet.
	restarted, err := NewValidatorRuntime(RuntimeConfig{
		Rules:       runtime.rules,
		State:       runtime.State(),
		Validators:  runtime.Validators(),
		VotingPower: runtime.votingPower,
		Threshold:   runtime.threshold,
		Proposer:    runtime.proposer,
	})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.State() != runtime.State() {
		t.Fatalf("restarted round state = %+v, want %+v", restarted.State(), runtime.State())
	}
	if len(restarted.Proposal()) != 0 || restarted.lockedProof != nil || len(restarted.lockedProposal) != 0 {
		t.Fatal("simulated restart invented non-persisted proposal/lock evidence")
	}

	staleA, err := NewTimeoutMessage(state, []byte("validator-a"), 1, signerA)
	if err != nil {
		t.Fatal(err)
	}
	staleB, err := NewTimeoutMessage(state, []byte("validator-b"), 1, signerB)
	if err != nil {
		t.Fatal(err)
	}
	before := restarted.State()
	if _, err := restarted.AdvanceRoundWithTimeoutEvidence([]Message{staleA, staleB}, resolver); err == nil {
		t.Fatal("stale pre-restart timeout evidence was accepted after simulated restart")
	}
	if restarted.State() != before {
		t.Fatal("restart-boundary replay mutated runtime state")
	}
}

func TestDeterministicRoundDriverFinalityBoundaryRemainsExplicit(t *testing.T) {
	runtime, state, _, _ := runtimeFixture(t)
	if err := runtime.AcceptProposal(runtimeMessage(state, "validator-a", MessageTypeProposal, "finalizable")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-a", MessageTypePrevote, "finalizable")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-b", MessageTypePrevote, "finalizable")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-a", MessageTypePrecommit, "finalizable")); err != nil {
		t.Fatal(err)
	}
	if err := runtime.AddVote(runtimeSignedMessage(t, state, "validator-b", MessageTypePrecommit, "finalizable")); err != nil {
		t.Fatal(err)
	}

	certificate, err := runtime.FinalizeProposal(runtimeTestAuthority(t))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.State().Phase != PhaseFinalized {
		t.Fatalf("phase = %v, want finalized", runtime.State().Phase)
	}
	if len(certificate.Votes) != 2 {
		t.Fatalf("finality evidence count = %d, want 2", len(certificate.Votes))
	}
	for _, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit || len(vote.Signature) == 0 {
			t.Fatalf("finality vote is not authenticated precommit: %+v", vote)
		}
	}

	// A finality certificate is an explicit handoff artifact; it is not itself
	// a canonical-state mutation. Node-side finalized-block tests own execution
	// and durable commit verification.
	if _, err := runtime.FinalizedCertificate(); err != nil {
		t.Fatal(err)
	}
}
