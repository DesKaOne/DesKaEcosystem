package consensus

import (
	"bytes"
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/storage"
)

func TestArtifactGCDecisionDurableReplayIsIdempotentAndAuthenticated(t *testing.T) {
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

	store := storage.NewMemoryArtifactGCDecisionStore()
	if err := PersistArtifactGCDecision(store, decision, validators, power, authority); err != nil {
		t.Fatal(err)
	}
	if err := PersistArtifactGCDecision(store, decision, validators, power, authority); err != nil {
		t.Fatal(err)
	}
	recovered, err := RecoverArtifactGCDecisions(store, validators, power, authority)
	if err != nil { t.Fatal(err) }
	if len(recovered) != 1 {
		t.Fatalf("recovered decisions = %d, want 1", len(recovered))
	}
	key, err := ArtifactGCDecisionKey(recovered[0])
	if err != nil { t.Fatal(err) }
	expected, err := ArtifactGCDecisionKey(decision)
	if err != nil { t.Fatal(err) }
	if key != expected {
		t.Fatalf("recovered key = %q, want %q", key, expected)
	}
}

func TestArtifactGCDecisionRecoveryRejectsTamperedOrCorruptRecords(t *testing.T) {
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
	encoded, err := EncodeArtifactGCDecision(decision)
	if err != nil { t.Fatal(err) }
	key, err := ArtifactGCDecisionKey(decision)
	if err != nil { t.Fatal(err) }

	store := storage.NewMemoryArtifactGCDecisionStore()
	if err := store.PutArtifactGCDecision(key, encoded); err != nil { t.Fatal(err) }
	decoded, err := DecodeArtifactGCDecision(encoded)
	if err != nil { t.Fatal(err) }
	decoded.Approvals[0].Signature[0] ^= 0xff
	tampered, err := EncodeArtifactGCDecision(decoded)
	if err != nil { t.Fatal(err) }
	if err := store.PutArtifactGCDecision(key, tampered); !errors.Is(err, storage.ErrArtifactGCDecisionConflict) {
		t.Fatalf("tampered overwrite error = %v", err)
	}

	if err := store.DeleteArtifactGCDecision(key); err != nil { t.Fatal(err) }
	if err := store.PutArtifactGCDecision(key, []byte("not-gob")); err != nil { t.Fatal(err) }
	if _, err := RecoverArtifactGCDecisions(store, validators, power, authority); !errors.Is(err, ErrCorruptGCDecision) {
		t.Fatalf("corrupt recovery error = %v, want %v", err, ErrCorruptGCDecision)
	}
}

func TestArtifactGCDecisionRecoveryIsDeterministicAcrossMultiplePlans(t *testing.T) {
	planA := testArtifactGCPlan()
	planB := planA
	planB.CandidateHeight = 89
	planB.CandidateHash = [32]byte{9, 8, 7}

	aID, bID, a, b, authority := testArtifactGCVoters(t)
	validators, err := NewValidatorSet([][]byte{aID, bID})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: aID, Power: 1},
		{ValidatorID: bID, Power: 1},
	})
	if err != nil { t.Fatal(err) }
	makeDecision := func(plan ArtifactGCPlan) ArtifactGCDecision {
		va, _ := BuildSignedArtifactGCVote(plan, aID, a)
		vb, _ := BuildSignedArtifactGCVote(plan, bID, b)
		d, _ := NewArtifactGCDecision(plan, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3}, []ArtifactGCVote{va, vb}, authority)
		return d
	}
	store := storage.NewMemoryArtifactGCDecisionStore()
	for _, decision := range []ArtifactGCDecision{makeDecision(planB), makeDecision(planA)} {
		if err := PersistArtifactGCDecision(store, decision, validators, power, authority); err != nil { t.Fatal(err) }
	}
	recovered, err := RecoverArtifactGCDecisions(store, validators, power, authority)
	if err != nil { t.Fatal(err) }
	if len(recovered) != 2 { t.Fatalf("recovered = %d, want 2", len(recovered)) }
	k0, _ := ArtifactGCDecisionKey(recovered[0])
	k1, _ := ArtifactGCDecisionKey(recovered[1])
	if bytes.Compare([]byte(k0), []byte(k1)) >= 0 {
		t.Fatalf("recovery order not deterministic: %q then %q", k0, k1)
	}
}

func TestArtifactGCDecisionFileStoreSurvivesReopen(t *testing.T) {
	plan := testArtifactGCPlan()
	aID, bID, a, b, authority := testArtifactGCVoters(t)
	validators, err := NewValidatorSet([][]byte{aID, bID})
	if err != nil { t.Fatal(err) }
	power, err := NewVotingPowerSet([]ValidatorVotingPower{{ValidatorID: aID, Power: 1}, {ValidatorID: bID, Power: 1}})
	if err != nil { t.Fatal(err) }
	va, _ := BuildSignedArtifactGCVote(plan, aID, a)
	vb, _ := BuildSignedArtifactGCVote(plan, bID, b)
	decision, err := NewArtifactGCDecision(plan, validators, power, QuorumThreshold{Numerator: 2, Denominator: 3}, []ArtifactGCVote{va, vb}, authority)
	if err != nil { t.Fatal(err) }
	path := t.TempDir() + "/gc.gob"
	store, err := storage.NewFileArtifactGCDecisionStore(path)
	if err != nil { t.Fatal(err) }
	if err := PersistArtifactGCDecision(store, decision, validators, power, authority); err != nil { t.Fatal(err) }
	reopened, err := storage.NewFileArtifactGCDecisionStore(path)
	if err != nil { t.Fatal(err) }
	recovered, err := RecoverArtifactGCDecisions(reopened, validators, power, authority)
	if err != nil { t.Fatal(err) }
	if len(recovered) != 1 { t.Fatalf("recovered after reopen = %d, want 1", len(recovered)) }
}
