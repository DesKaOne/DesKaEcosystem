package consensus

import (
	"bytes"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

func testArtifactGCPlan() ArtifactGCPlan {
	return ArtifactGCPlan{
		ProtocolVersion: 1,
		ChainID:         1,
		CurrentEpoch:    4,
		CanonicalHeight: 100,
		CandidateHeight: 90,
		CandidateHash:   [32]byte{1, 2, 3},
		Policy: ArtifactRetentionPolicy{
			KeepRecentHeights: 5,
			KeepRecentEpochs: 1,
		},
		EvidenceKeys: []string{"a", "b"},
	}
}

func testArtifactGCVoters(t *testing.T) ([]byte, []byte, crypto.Signer, crypto.Signer, StaticValidatorAuthority) {
	t.Helper()
	seedA := bytes.Repeat([]byte{1}, 32)
	seedB := bytes.Repeat([]byte{2}, 32)
	a, err := crypto.NewEd25519KeyPair(seedA)
	if err != nil { t.Fatal(err) }
	b, err := crypto.NewEd25519KeyPair(seedB)
	if err != nil { t.Fatal(err) }
	authority, err := NewStaticValidatorAuthority(map[string][]byte{
		"a": a.PublicKey,
		"b": b.PublicKey,
	})
	if err != nil { t.Fatal(err) }
	return []byte("a"), []byte("b"), a, b, authority
}

func TestArtifactGCPlanDigestIsDeterministicAndOrderSensitive(t *testing.T) {
	plan := testArtifactGCPlan()
	first, err := plan.Digest()
	if err != nil { t.Fatal(err) }
	second, err := plan.Digest()
	if err != nil { t.Fatal(err) }
	if first != second { t.Fatal("plan digest changed across identical evaluations") }

	changed := plan
	changed.EvidenceKeys = []string{"b", "a"}
	if _, err := changed.Digest(); err == nil {
		t.Fatal("unsorted evidence keys accepted")
	}
}

func TestArtifactGCDecisionRequiresAuthenticatedQuorum(t *testing.T) {
	plan := testArtifactGCPlan()
	aID, bID, a, b, authority := testArtifactGCVoters(t)
	validators, err := NewValidatorSet([][]byte{aID, bID})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: aID, Power: 1},
		{ValidatorID: bID, Power: 1},
	})
	if err != nil { t.Fatal(err) }

	voteA, err := BuildSignedArtifactGCVote(plan, aID, a)
	if err != nil { t.Fatal(err) }
	voteB, err := BuildSignedArtifactGCVote(plan, bID, b)
	if err != nil { t.Fatal(err) }

	decision, err := NewArtifactGCDecision(
		plan, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		[]ArtifactGCVote{voteA, voteB}, authority,
	)
	if err != nil { t.Fatal(err) }
	if err := decision.Validate(validators, power, authority); err != nil {
		t.Fatal(err)
	}
	if len(decision.Approvals) != 2 {
		t.Fatalf("approvals = %d, want 2", len(decision.Approvals))
	}

	tampered := voteB
	tampered.PlanDigest[0] ^= 0xff
	if _, err := NewArtifactGCDecision(
		plan, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		[]ArtifactGCVote{voteA, tampered}, authority,
	); err != ErrArtifactGCPlanMismatch {
		t.Fatalf("tampered vote error = %v, want %v", err, ErrArtifactGCPlanMismatch)
	}
}

func TestArtifactGCDecisionRejectsDuplicateAndInsufficientVotes(t *testing.T) {
	plan := testArtifactGCPlan()
	aID, _, a, _, authority := testArtifactGCVoters(t)
	validators, err := NewValidatorSet([][]byte{aID})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{{ValidatorID: aID, Power: 1}})
	if err != nil { t.Fatal(err) }
	vote, err := BuildSignedArtifactGCVote(plan, aID, a)
	if err != nil { t.Fatal(err) }

	if _, err := NewArtifactGCDecision(
		plan, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3},
		[]ArtifactGCVote{vote}, authority,
	); err != ErrArtifactGCQuorumNotReached {
		t.Fatalf("insufficient quorum error = %v, want %v", err, ErrArtifactGCQuorumNotReached)
	}

	if _, err := NewArtifactGCDecision(
		plan, validators, power, QuorumThreshold{Numerator: 1, Denominator: 1},
		[]ArtifactGCVote{vote, vote}, authority,
	); err != ErrDuplicateArtifactGCVote {
		t.Fatalf("duplicate vote error = %v, want %v", err, ErrDuplicateArtifactGCVote)
	}
}
