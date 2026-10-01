package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"sort"
)

var (
	ErrNilGCDecisionStore       = errors.New("nil artifact GC decision store")
	ErrInvalidGCDecisionKey     = errors.New("invalid artifact GC decision key")
	ErrConflictingGCDecision    = errors.New("conflicting artifact GC decision")
	ErrCorruptGCDecision        = errors.New("corrupt artifact GC decision")
)

// GCDecisionStore persists coordinated artifact-GC decisions separately from
// canonical ChainStore and consensus evidence. It is an audit/recovery
// boundary only; loading a decision never performs cleanup.
type GCDecisionStore interface {
	PutArtifactGCDecision(key string, encoded []byte) error
	LoadArtifactGCDecisions() (map[string][]byte, error)
	DeleteArtifactGCDecision(key string) error
}

// ArtifactGCDecisionKey returns the stable plan digest used as the durable
// identity of one coordinated cleanup decision.
func ArtifactGCDecisionKey(decision ArtifactGCDecision) (string, error) {
	digest, err := decision.Plan.Digest()
	if err != nil {
		return "", err
	}
	return hexDigest(digest), nil
}

// EncodeArtifactGCDecision serializes a validated decision for durable audit.
func EncodeArtifactGCDecision(decision ArtifactGCDecision) ([]byte, error) {
	if err := decision.Plan.Validate(); err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(decision); err != nil {
		return nil, ErrCorruptGCDecision
	}
	return b.Bytes(), nil
}

// DecodeArtifactGCDecision decodes one durable decision without trusting it.
// Caller must run ValidateArtifactGCDecision after decoding.
func DecodeArtifactGCDecision(encoded []byte) (ArtifactGCDecision, error) {
	if len(encoded) == 0 {
		return ArtifactGCDecision{}, ErrCorruptGCDecision
	}
	var decision ArtifactGCDecision
	if err := gob.NewDecoder(bytes.NewReader(encoded)).Decode(&decision); err != nil {
		return ArtifactGCDecision{}, ErrCorruptGCDecision
	}
	if _, err := ArtifactGCDecisionKey(decision); err != nil {
		return ArtifactGCDecision{}, err
	}
	return decision, nil
}

// PersistArtifactGCDecision validates the complete quorum decision before
// writing it. Identical replay is idempotent; same plan identity with
// different bytes is rejected as a conflict.
func PersistArtifactGCDecision(
	store GCDecisionStore,
	decision ArtifactGCDecision,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	authority TimeoutAuthorityResolver,
) error {
	if store == nil {
		return ErrNilGCDecisionStore
	}
	if err := decision.Validate(validators, votingPower, authority); err != nil {
		return err
	}
	key, err := ArtifactGCDecisionKey(decision)
	if err != nil {
		return err
	}
	encoded, err := EncodeArtifactGCDecision(decision)
	if err != nil {
		return err
	}
	existing, err := store.LoadArtifactGCDecisions()
	if err != nil {
		return err
	}
	if prior, ok := existing[key]; ok {
		if bytes.Equal(prior, encoded) {
			return nil
		}
		return ErrConflictingGCDecision
	}
	return store.PutArtifactGCDecision(key, encoded)
}

// RecoverArtifactGCDecisions loads every durable decision and validates its
// identity, validator membership, signatures, voting power and quorum.
// Results are deterministic and recovery never mutates consensus or artifacts.
func RecoverArtifactGCDecisions(
	store GCDecisionStore,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	authority TimeoutAuthorityResolver,
) ([]ArtifactGCDecision, error) {
	if store == nil {
		return nil, ErrNilGCDecisionStore
	}
	records, err := store.LoadArtifactGCDecisions()
	if err != nil {
		return nil, err
	}
	decisions := make([]ArtifactGCDecision, 0, len(records))
	for key, encoded := range records {
		decision, err := DecodeArtifactGCDecision(encoded)
		if err != nil {
			return nil, err
		}
		computed, err := ArtifactGCDecisionKey(decision)
		if err != nil || computed != key {
			return nil, ErrConflictingGCDecision
		}
		if err := decision.Validate(validators, votingPower, authority); err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	sort.Slice(decisions, func(i, j int) bool {
		ki, _ := ArtifactGCDecisionKey(decisions[i])
		kj, _ := ArtifactGCDecisionKey(decisions[j])
		return ki < kj
	})
	return decisions, nil
}

func hexDigest(digest [32]byte) string {
	const hex = "0123456789abcdef"
	out := make([]byte, 64)
	for i, b := range digest {
		out[i*2] = hex[b>>4]
		out[i*2+1] = hex[b&0x0f]
	}
	return string(out)
}

var _ = sha256.Sum256
