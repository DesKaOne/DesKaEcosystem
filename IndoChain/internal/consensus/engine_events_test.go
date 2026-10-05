package consensus

import (
 "testing"
 "time"
 "github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func TestConsensusEngineProcessMessageGeneratesAndConsumesLocalVotes(t *testing.T) {
 state, err := NewRoundState(1, 2002, 1, 3); if err != nil { t.Fatal(err) }
 signer, err := crypto.GenerateEd25519Signer(); if err != nil { t.Fatal(err) }
 validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")}); if err != nil { t.Fatal(err) }
 power, err := NewVotingPowerSet([]ValidatorVotingPower{{ValidatorID: []byte("validator-a"), Power: 1},{ValidatorID: []byte("validator-b"), Power: 1}}); if err != nil { t.Fatal(err) }
 authority, err := NewStaticValidatorAuthority(map[string][]byte{"validator-a": signer.PublicKey(),"validator-b": signer.PublicKey()}); if err != nil { t.Fatal(err) }
 runtime, err := NewValidatorRuntime(RuntimeConfig{
  Rules: ValidationRules{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, RequireSender: true},
  State: state, Validators: validators, VotingPower: power,
  Threshold: QuorumThreshold{Numerator: 2, Denominator: 3}, Proposer: RoundRobinProposer{},
 }); if err != nil { t.Fatal(err) }
 engine, err := NewConsensusEngine(runtime, authority, []byte("validator-a"), signer, TimeoutPolicy{Proposal: time.Second, Prevote: time.Second, Precommit: time.Second}); if err != nil { t.Fatal(err) }

 proposal := Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: []byte("validator-a"), Type: MessageTypeProposal, Payload: []byte("candidate-hash")}
 proposal, err = proposal.Sign(signer); if err != nil { t.Fatal(err) }
 events, err := engine.ProcessMessage(proposal); if err != nil { t.Fatal(err) }
 if len(events) != 1 || events[0].Type != MessageTypePrevote { t.Fatalf("proposal events = %+v", events) }
 if runtime.State().Phase != PhasePrevote { t.Fatalf("phase after local prevote = %v", runtime.State().Phase) }

 remotePrevote := Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: []byte("validator-b"), Type: MessageTypePrevote, Payload: []byte("candidate-hash")}
 remotePrevote, err = remotePrevote.Sign(signer); if err != nil { t.Fatal(err) }
 events, err = engine.ProcessMessage(remotePrevote); if err != nil { t.Fatal(err) }
 if len(events) != 1 || events[0].Type != MessageTypePrecommit { t.Fatalf("prevote events = %+v", events) }
 if runtime.State().Phase != PhasePrecommit { t.Fatalf("phase after prevote quorum = %v", runtime.State().Phase) }

 remotePrecommit := Message{ProtocolVersion: state.ProtocolVersion, ChainID: state.ChainID, Epoch: state.Epoch, Height: state.Height, Round: state.Round, Sender: []byte("validator-b"), Type: MessageTypePrecommit, Payload: []byte("candidate-hash")}
 remotePrecommit, err = remotePrecommit.Sign(signer); if err != nil { t.Fatal(err) }
 if _, err := engine.ProcessMessage(remotePrecommit); err != nil { t.Fatal(err) }
 if runtime.State().Phase != PhaseFinalized { t.Fatalf("phase after precommit quorum = %v", runtime.State().Phase) }
 if _, err := runtime.FinalizedCertificate(); err != nil { t.Fatalf("finality certificate missing: %v", err) }
}
