package consensus

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestConsensusEngineOwnsTimeoutGenerationAndRejectsStaleToken(t *testing.T) {
	state, err := NewRoundState(1, 1001, 1, 7)
	if err != nil { t.Fatal(err) }
	signer, err := crypto.GenerateEd25519Signer()
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil { t.Fatal(err) }
	authority, err := NewStaticValidatorAuthority(map[string][]byte{
		"validator-a": signer.PublicKey(),
		"validator-b": signer.PublicKey(),
	})
	if err != nil { t.Fatal(err) }

	runtime, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }

	policy := TimeoutPolicy{Proposal: time.Second, Prevote: 2 * time.Second, Precommit: 3 * time.Second}
	engine, err := NewConsensusEngine(runtime, authority, []byte("validator-a"), signer, policy)
	if err != nil { t.Fatal(err) }

	first, duration, err := engine.ArmTimeout()
	if err != nil { t.Fatal(err) }
	if duration != policy.Proposal { t.Fatalf("proposal timeout = %v", duration) }

	proposal := Message{
		ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID,
		Epoch: state.Epoch, Height: state.Height, Round: state.Round,
		Sender: []byte("validator-a"), Type: MessageTypeProposal,
		Payload: []byte("proposal"),
	}
	proposal, err = proposal.Sign(signer)
	if err != nil { t.Fatal(err) }
	if err := engine.HandleMessage(proposal); err != nil { t.Fatal(err) }

	second, duration, err := engine.ArmTimeout()
	if err != nil { t.Fatal(err) }
	if duration != policy.Prevote { t.Fatalf("prevote timeout = %v", duration) }
	if second.Generation == first.Generation {
		t.Fatal("timeout generation did not advance")
	}

	if _, err := engine.HandleTimeout(first); !errors.Is(err, ErrStaleConsensusTimeout) {
		t.Fatalf("stale timeout error = %v", err)
	}
}

func TestConsensusEngineTimeoutQuorumAdvancesRoundAndRearmsProposal(t *testing.T) {
	state, err := NewRoundState(1, 1001, 1, 7)
	if err != nil { t.Fatal(err) }
	signer, err := crypto.GenerateEd25519Signer()
	if err != nil { t.Fatal(err) }
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("validator-a"), Power: 1},
		{ValidatorID: []byte("validator-b"), Power: 1},
	})
	if err != nil { t.Fatal(err) }
	authority, err := NewStaticValidatorAuthority(map[string][]byte{
		"validator-a": signer.PublicKey(),
		"validator-b": signer.PublicKey(),
	})
	if err != nil { t.Fatal(err) }

	runtime, err := NewValidatorRuntime(RuntimeConfig{
		Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
		State: state, Validators: validators, VotingPower: power,
		Threshold: QuorumThreshold{Numerator: 2, Denominator: 3},
		Proposer: RoundRobinProposer{},
	})
	if err != nil { t.Fatal(err) }
	engine, err := NewConsensusEngine(runtime, authority, []byte("validator-a"), signer, TimeoutPolicy{
		Proposal: time.Second, Prevote: time.Second, Precommit: time.Second,
	})
	if err != nil { t.Fatal(err) }

	if _, _, err := engine.ArmTimeout(); err != nil { t.Fatal(err) }
	localTimeout, err := engine.HandleTimeout(TimeoutToken{
		Height: uint64(state.Height), Round: state.Round, Phase: PhaseProposal, Generation: 1,
	})
	if err != nil { t.Fatal(err) }

	remoteTimeout, err := NewTimeoutMessage(
		state, []byte("validator-b"), state.Round+1, signer,
	)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(remoteTimeout.Payload, localTimeout.Payload) {
		t.Fatal("timeout evidence payload diverged for equivalent validators")
	}
	if err := engine.HandleMessage(remoteTimeout); err != nil { t.Fatal(err) }

	certificate, err := engine.TryAdvanceRound()
	if err != nil { t.Fatal(err) }
	if certificate.NextRound != 1 { t.Fatalf("next round = %d", certificate.NextRound) }
	if runtime.State().Round != 1 || runtime.State().Phase != PhaseProposal {
		t.Fatalf("runtime after timeout quorum = %+v", runtime.State())
	}
	if !engine.ArmedTimeout() { t.Fatal("proposal timeout was not rearmed after round change") }
}
