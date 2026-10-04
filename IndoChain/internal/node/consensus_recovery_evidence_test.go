package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestReconstructConsensusRuntimeWithEvidenceRecoversAuthenticatedRecords(t *testing.T) {
	chainStore := storage.NewMemoryStore()
	n, err := NewDevnet(chainStore)
	if err != nil { t.Fatal(err) }

	validators, power := recoveryValidatorConfig(t)
	id := []byte("validator-a")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil { t.Fatal(err) }
	authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{string(id): publicKey})
	if err != nil { t.Fatal(err) }

	state, err := consensus.NewRoundState(n.Config.ProtocolVersion, n.Config.ChainID, 7, n.Head.Header.Height)
	if err != nil { t.Fatal(err) }
	msg := consensus.Message{
		ProtocolVersion: state.ProtocolVersion,
		ChainID: state.ChainID,
		Epoch: state.Epoch,
		Height: state.Height,
		Round: 2,
		Sender: id,
		Type: consensus.MessageTypePrevote,
		Payload: []byte("proposal-hash"),
	}
	msg.Signature = ed25519.Sign(privateKey, msg.SigningBytes())

	evidenceStore := storage.NewMemoryConsensusEvidenceStore()
	persistState := state
	persistState.Round = msg.Round
	if err := consensus.PersistAuthenticatedEvidence(evidenceStore, msg, persistState, validators, authority); err != nil {
		t.Fatal(err)
	}

	recovered, err := n.ReconstructConsensusRuntimeWithEvidence(
		7, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		consensus.RoundRobinProposer{},
		evidenceStore, authority,
	)
	if err != nil { t.Fatal(err) }
	if recovered.Runtime == nil || len(recovered.Evidence) != 1 {
		t.Fatalf("recovered runtime/evidence = %#v / %d", recovered.Runtime, len(recovered.Evidence))
	}
	if recovered.Evidence[0].Round != 2 || recovered.Evidence[0].Type != consensus.MessageTypePrevote {
		t.Fatalf("unexpected recovered evidence: %+v", recovered.Evidence[0])
	}
	if recovered.Height != n.Head.Header.Height || recovered.CanonicalHash != n.HeadHash {
		t.Fatal("evidence recovery changed canonical recovery source")
	}
	if got := recovered.Runtime.State(); got.Phase != consensus.PhaseProposal || got.Round != 0 {
		t.Fatalf("runtime was implicitly advanced by evidence recovery: %+v", got)
	}
}
