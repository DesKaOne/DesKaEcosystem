package consensus

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/crypto"
)

const artifactGCSigningDomain = "INDOCHAIN-ARTIFACT-GC"

var (
	ErrInvalidArtifactGCPlan = errors.New("invalid artifact GC plan")
	ErrArtifactGCPlanMismatch = errors.New("artifact GC plan mismatch")
	ErrInvalidArtifactGCVote = errors.New("invalid artifact GC vote")
	ErrArtifactGCQuorumNotReached = errors.New("artifact GC quorum not reached")
	ErrDuplicateArtifactGCVote = errors.New("duplicate artifact GC vote")
)

// ArtifactGCPlan is the deterministic identity of one coordinated historical
// artifact cleanup request. It is advisory to consensus: it can authorize
// local cleanup only and can never make a block canonical.
type ArtifactGCPlan struct {
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	CurrentEpoch    uint64
	CanonicalHeight types.Height
	CandidateHeight types.Height
	CandidateHash   types.Hash
	Policy          ArtifactRetentionPolicy
	EvidenceKeys    []string
}

func (p ArtifactGCPlan) Validate() error {
	if p.ProtocolVersion == 0 || p.ChainID == 0 {
		return ErrInvalidArtifactGCPlan
	}
	if err := p.Policy.Validate(); err != nil {
		return err
	}
	if p.CandidateHeight > p.CanonicalHeight || p.CandidateHash == (types.Hash{}) {
		return ErrInvalidArtifactGCPlan
	}
	for i, key := range p.EvidenceKeys {
		if key == "" || (i > 0 && p.EvidenceKeys[i-1] >= key) {
			return ErrInvalidArtifactGCPlan
		}
	}
	return nil
}

// Digest returns the stable plan identity signed by validators.
func (p ArtifactGCPlan) Digest() ([32]byte, error) {
	if err := p.Validate(); err != nil {
		return [32]byte{}, err
	}
	var b bytes.Buffer
	putGCU16(&b, uint16(p.ProtocolVersion))
	putGCU64(&b, uint64(p.ChainID))
	putGCU64(&b, p.CurrentEpoch)
	putGCU64(&b, uint64(p.CanonicalHeight))
	putGCU64(&b, uint64(p.CandidateHeight))
	b.Write(p.CandidateHash[:])
	putGCU64(&b, p.Policy.KeepRecentHeights)
	putGCU64(&b, p.Policy.KeepRecentEpochs)
	putGCU32(&b, uint32(len(p.EvidenceKeys)))
	for _, key := range p.EvidenceKeys {
		putGCBytes(&b, []byte(key))
	}
	return sha256.Sum256(b.Bytes()), nil
}

// ArtifactGCVote is one validator's authenticated approval of an exact plan.
type ArtifactGCVote struct {
	PlanDigest [32]byte
	Sender     []byte
	Signature  []byte
}

func (v ArtifactGCVote) SigningBytes() []byte {
	var b bytes.Buffer
	b.Write(v.PlanDigest[:])
	putGCBytes(&b, v.Sender)
	return crypto.DomainSeparatedMessage(artifactGCSigningDomain, 1, b.Bytes())
}

func BuildSignedArtifactGCVote(plan ArtifactGCPlan, sender []byte, signer crypto.Signer) (ArtifactGCVote, error) {
	if err := plan.Validate(); err != nil {
		return ArtifactGCVote{}, err
	}
	if len(sender) == 0 || signer == nil {
		return ArtifactGCVote{}, ErrInvalidArtifactGCVote
	}
	digest, err := plan.Digest()
	if err != nil {
		return ArtifactGCVote{}, err
	}
	vote := ArtifactGCVote{PlanDigest: digest, Sender: append([]byte(nil), sender...)}
	sig, err := signer.Sign(vote.SigningBytes())
	if err != nil {
		return ArtifactGCVote{}, err
	}
	vote.Signature = append([]byte(nil), sig...)
	return vote, nil
}

func ValidateArtifactGCVote(
	vote ArtifactGCVote,
	plan ArtifactGCPlan,
	validators ValidatorSet,
	authority TimeoutAuthorityResolver,
) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	if len(vote.Sender) == 0 || len(vote.Signature) == 0 || authority == nil {
		return ErrInvalidArtifactGCVote
	}
	digest, err := plan.Digest()
	if err != nil {
		return err
	}
	if vote.PlanDigest != digest {
		return ErrArtifactGCPlanMismatch
	}
	if err := validators.Require(vote.Sender); err != nil {
		return err
	}
	key, err := authority.PublicKeyForValidator(vote.Sender)
	if err != nil {
		return err
	}
	if err := crypto.VerifyEd25519(vote.SigningBytes(), vote.Signature, key); !err {
		return ErrInvalidSignature
	}
	return nil
}

// ArtifactGCDecision is a quorum-backed cleanup authorization for one exact
// plan. It is deliberately not a consensus finality certificate.
type ArtifactGCDecision struct {
	Plan      ArtifactGCPlan
	Threshold QuorumThreshold
	Approvals []ArtifactGCVote
}

func NewArtifactGCDecision(
	plan ArtifactGCPlan,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	threshold QuorumThreshold,
	approvals []ArtifactGCVote,
	authority TimeoutAuthorityResolver,
) (ArtifactGCDecision, error) {
	if err := plan.Validate(); err != nil {
		return ArtifactGCDecision{}, err
	}
	if err := validators.Validate(); err != nil {
		return ArtifactGCDecision{}, err
	}
	if err := votingPower.Validate(); err != nil {
		return ArtifactGCDecision{}, err
	}
	if err := threshold.Validate(); err != nil {
		return ArtifactGCDecision{}, err
	}
	if authority == nil {
		return ArtifactGCDecision{}, ErrInvalidArtifactGCVote
	}

	ordered := append([]ArtifactGCVote(nil), approvals...)
	sort.Slice(ordered, func(i, j int) bool {
		return bytes.Compare(ordered[i].Sender, ordered[j].Sender) < 0
	})
	var votedPower uint64
	seen := make(map[string]struct{}, len(ordered))
	for _, vote := range ordered {
		if err := ValidateArtifactGCVote(vote, plan, validators, authority); err != nil {
			return ArtifactGCDecision{}, err
		}
		id := string(vote.Sender)
		if _, ok := seen[id]; ok {
			return ArtifactGCDecision{}, ErrDuplicateArtifactGCVote
		}
		seen[id] = struct{}{}
		power, ok := votingPower.PowerOf(vote.Sender)
		if !ok {
			return ArtifactGCDecision{}, ErrValidatorNotFound
		}
		if ^uint64(0)-votedPower < power {
			return ArtifactGCDecision{}, ErrInvalidVotingPowerSet
		}
		votedPower += power
	}
	totalPower, err := votingPower.TotalPower()
	if err != nil {
		return ArtifactGCDecision{}, err
	}
	reached, err := QuorumReached(votedPower, totalPower, threshold)
	if err != nil {
		return ArtifactGCDecision{}, err
	}
	if !reached {
		return ArtifactGCDecision{}, ErrArtifactGCQuorumNotReached
	}
	return ArtifactGCDecision{
		Plan: plan, Threshold: threshold, Approvals: ordered,
	}, nil
}

func (d ArtifactGCDecision) Validate(
	validators ValidatorSet,
	votingPower VotingPowerSet,
	authority TimeoutAuthorityResolver,
) error {
	_, err := NewArtifactGCDecision(d.Plan, validators, votingPower, d.Threshold, d.Approvals, authority)
	return err
}

func putGCU16(b *bytes.Buffer, v uint16) {
	var x [2]byte
	binary.BigEndian.PutUint16(x[:], v)
	b.Write(x[:])
}

func putGCU32(b *bytes.Buffer, v uint32) {
	var x [4]byte
	binary.BigEndian.PutUint32(x[:], v)
	b.Write(x[:])
}

func putGCU64(b *bytes.Buffer, v uint64) {
	var x [8]byte
	binary.BigEndian.PutUint64(x[:], v)
	b.Write(x[:])
}

func putGCBytes(b *bytes.Buffer, v []byte) {
	putGCU32(b, uint32(len(v)))
	b.Write(v)
}
