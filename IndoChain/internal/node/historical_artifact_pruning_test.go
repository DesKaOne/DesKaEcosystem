package node

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func testNodeWithCanonicalHeight(t *testing.T, height types.Height) *Node {
	t.Helper()
	n, err := NewDevnet(storage.NewMemoryStore())
	if err != nil {
		t.Fatal(err)
	}
	n.Head.Header.Height = height
	n.HeadHash = types.Hash{1}
	return n
}

func TestPruneHistoricalFinalityArtifactsRejectsRetainedArtifactAtomically(t *testing.T) {
	n := testNodeWithCanonicalHeight(t, 10)
	candidate := block.Block{Header: block.Header{Height: 7, Timestamp: 1}}
	hash, err := block.Hash(candidate)
	if err != nil { t.Fatal(err) }
	candidates := storage.NewMemoryCandidateStore()
	if err := candidates.SaveCandidate(storage.CandidateKey{Height: 7, Hash: hash}, candidate); err != nil {
		t.Fatal(err)
	}
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()
	msg := consensus.Message{ProtocolVersion: n.Config.ProtocolVersion, ChainID: n.Config.ChainID, Epoch: 5, Height: 7, Round: 0, Sender: []byte("validator")}
	policy := consensus.ArtifactRetentionPolicy{KeepRecentHeights: 5, KeepRecentEpochs: 2}

	_, err = n.PruneHistoricalFinalityArtifacts(policy, 5, candidates, evidenceStore, storage.CandidateKey{Height: 7, Hash: hash}, []consensus.Message{msg})
	if !errors.Is(err, ErrHistoricalArtifactsRetained) {
		t.Fatalf("expected retained artifact rejection, got %v", err)
	}
	if _, err := candidates.GetCandidate(storage.CandidateKey{Height: 7, Hash: hash}); err != nil {
		t.Fatalf("candidate was modified on rejected policy: %v", err)
	}
}

func TestPruneHistoricalFinalityArtifactsPrunesOnlyAfterBothWindows(t *testing.T) {
	n := testNodeWithCanonicalHeight(t, 10)
	candidate := block.Block{Header: block.Header{Height: 7, Timestamp: 1}}
	hash, err := block.Hash(candidate)
	if err != nil { t.Fatal(err) }
	candidates := storage.NewMemoryCandidateStore()
	key := storage.CandidateKey{Height: 7, Hash: hash}
	if err := candidates.SaveCandidate(key, candidate); err != nil { t.Fatal(err) }

	evidenceStore := storage.NewMemoryConsensusEvidenceStore()
	msg := consensus.Message{
		ProtocolVersion: n.Config.ProtocolVersion,
		ChainID: n.Config.ChainID,
		Epoch: 2,
		Height: 7,
		Round: 0,
		Sender: []byte("validator"),
		Type: consensus.MessageTypeVote,
	}
	evidenceKey, err := consensus.ConsensusEvidenceKey(msg)
	if err != nil { t.Fatal(err) }
	if err := evidenceStore.PutConsensusEvidence(evidenceKey, []byte("opaque")); err != nil { t.Fatal(err) }

	policy := consensus.ArtifactRetentionPolicy{KeepRecentHeights: 3, KeepRecentEpochs: 2}
	_, err = n.PruneHistoricalFinalityArtifacts(policy, 5, candidates, evidenceStore, key, []consensus.Message{msg})
	if err != nil {
		t.Fatalf("expected pruning after both windows, got %v", err)
	}
	if _, err := candidates.GetCandidate(key); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("expected candidate to be pruned, got %v", err)
	}
	records, err := evidenceStore.LoadConsensusEvidence()
	if err != nil { t.Fatal(err) }
	if len(records) != 0 { t.Fatalf("expected evidence to be pruned, got %d records", len(records)) }
}
