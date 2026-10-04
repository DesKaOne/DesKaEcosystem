package node

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/consensus"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/block"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func testCoordinatedGCDecision(t *testing.T, plan consensus.ArtifactGCPlan) (consensus.ArtifactGCDecision, consensus.ValidatorSet, consensus.VotingPowerSet, consensus.StaticValidatorAuthority) {
	t.Helper()
	a, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{1}, 32))
	if err != nil { t.Fatal(err) }
	b, err := crypto.NewEd25519KeyPair(bytes.Repeat([]byte{2}, 32))
	if err != nil { t.Fatal(err) }
	authority, err := consensus.NewStaticValidatorAuthority(map[string][]byte{"a": a.PublicKey, "b": b.PublicKey})
	if err != nil { t.Fatal(err) }
	validators, err := consensus.NewValidatorSet([][]byte{[]byte("a"), []byte("b")})
	if err != nil { t.Fatal(err) }
	power, err := consensus.NewVotingPowerSet([]consensus.ValidatorVotingPower{
		{ValidatorID: []byte("a"), Power: 1},
		{ValidatorID: []byte("b"), Power: 1},
	})
	if err != nil { t.Fatal(err) }
	voteA, err := consensus.BuildSignedArtifactGCVote(plan, []byte("a"), a)
	if err != nil { t.Fatal(err) }
	voteB, err := consensus.BuildSignedArtifactGCVote(plan, []byte("b"), b)
	if err != nil { t.Fatal(err) }
	decision, err := consensus.NewArtifactGCDecision(
		plan, validators, power,
		consensus.QuorumThreshold{Numerator: 2, Denominator: 3},
		[]consensus.ArtifactGCVote{voteA, voteB}, authority,
	)
	if err != nil { t.Fatal(err) }
	return decision, validators, power, authority
}

func TestApplyCoordinatedArtifactGCDeletesOnlyAuthorizedCandidate(t *testing.T) {
	n := testNodeWithCanonicalHeight(t, 10)
	candidate := block.Block{Header: block.Header{Height: 7, Timestamp: 1}}
	hash, err := block.Hash(candidate)
	if err != nil { t.Fatal(err) }
	candidates := storage.NewMemoryCandidateStore()
	key := storage.CandidateKey{Height: 7, Hash: hash}
	if err := candidates.SaveCandidate(key, candidate); err != nil { t.Fatal(err) }
	evidenceStore := storage.NewMemoryConsensusEvidenceStore()

	plan := consensus.ArtifactGCPlan{
		ProtocolVersion: n.Config.ProtocolVersion,
		ChainID: n.Config.ChainID,
		CurrentEpoch: 5,
		CanonicalHeight: 10,
		CandidateHeight: 7,
		CandidateHash: hash,
		Policy: consensus.ArtifactRetentionPolicy{KeepRecentHeights: 3},
	}
	decision, validators, power, authority := testCoordinatedGCDecision(t, plan)
	headHash := n.HeadHash

	if _, err := n.ApplyCoordinatedArtifactGC(
		decision, validators, power, authority, candidates, evidenceStore, nil,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := candidates.GetCandidate(key); !errors.Is(err, storage.ErrCandidateNotFound) {
		t.Fatalf("candidate was not pruned: %v", err)
	}
	if n.Head.Header.Height != 10 || n.HeadHash != headHash {
		t.Fatal("coordinated GC mutated canonical head")
	}
}

func TestApplyCoordinatedArtifactGCRejectsLocalEvidenceSetMismatch(t *testing.T) {
	n := testNodeWithCanonicalHeight(t, 10)
	plan := consensus.ArtifactGCPlan{
		ProtocolVersion: n.Config.ProtocolVersion,
		ChainID: n.Config.ChainID,
		CurrentEpoch: 5,
		CanonicalHeight: 10,
		CandidateHeight: 7,
		CandidateHash: types.Hash{1},
		Policy: consensus.ArtifactRetentionPolicy{KeepRecentHeights: 3},
		EvidenceKeys: []string{"authorized"},
	}
	decision, validators, power, authority := testCoordinatedGCDecision(t, plan)
	_, err := n.ApplyCoordinatedArtifactGC(
		decision, validators, power, authority,
		storage.NewMemoryCandidateStore(),
		storage.NewMemoryConsensusEvidenceStore(),
		nil,
	)
	if !errors.Is(err, ErrCoordinatedGCEvidenceMismatch) {
		t.Fatalf("evidence mismatch error = %v", err)
	}
}
