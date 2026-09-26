package consensus

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidPrecommitCertificate = errors.New("invalid consensus precommit certificate")
	ErrPrecommitQuorumNotReached   = errors.New("precommit quorum not reached")
	ErrInvalidLockProof             = errors.New("invalid consensus lock proof")
)

// PrecommitCertificate is an explicit quorum certificate for precommit votes
// for one exact consensus context and one opaque proposal payload.
//
// It is a development proof-of-lock boundary. It does not define the
// production BFT locking algorithm or signature aggregation scheme.
type PrecommitCertificate struct {
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	Epoch           uint64
	Height          types.Height
	Round           uint64
	Payload         []byte
	Threshold       QuorumThreshold
	Votes           []Message
}

// NewPrecommitCertificate constructs deterministic precommit evidence from
// explicit MessageTypePrecommit votes. Votes are canonically ordered by sender.
func NewPrecommitCertificate(
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	threshold QuorumThreshold,
	payload []byte,
	votes []Message,
) (PrecommitCertificate, error) {
	if err := state.Validate(); err != nil {
		return PrecommitCertificate{}, err
	}
	if err := threshold.Validate(); err != nil {
		return PrecommitCertificate{}, err
	}
	if len(payload) == 0 {
		return PrecommitCertificate{}, ErrInvalidPrecommitCertificate
	}

	aggregator, err := NewVoteAggregator(
		ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			RequireSender:   true,
		},
		state,
		validators,
		votingPower,
	)
	if err != nil {
		return PrecommitCertificate{}, err
	}

	for _, vote := range votes {
		if vote.Type != MessageTypePrecommit {
			return PrecommitCertificate{}, ErrInvalidPrecommitCertificate
		}
		if err := aggregator.AddVote(vote); err != nil {
			return PrecommitCertificate{}, fmt.Errorf("add precommit vote: %w", err)
		}
	}

	quorum, err := aggregator.QuorumForPayload(payload, threshold)
	if err != nil {
		return PrecommitCertificate{}, err
	}
	if !quorum {
		return PrecommitCertificate{}, ErrPrecommitQuorumNotReached
	}

	collected := cloneVotes(aggregator.VotesForPayload(payload))
	sort.Slice(collected, func(i, j int) bool {
		return bytes.Compare(collected[i].Sender, collected[j].Sender) < 0
	})

	return PrecommitCertificate{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		Payload:         append([]byte(nil), payload...),
		Threshold:       threshold,
		Votes:           collected,
	}, nil
}

// ValidatePrecommitCertificate independently validates precommit evidence.
// Validation is non-mutating and requires canonical sender ordering.
func ValidatePrecommitCertificate(
	certificate PrecommitCertificate,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
) error {
	if err := certificate.validateContext(state); err != nil {
		return err
	}
	if err := certificate.Threshold.Validate(); err != nil {
		return err
	}
	if len(certificate.Payload) == 0 || len(certificate.Votes) == 0 {
		return ErrInvalidPrecommitCertificate
	}

	aggregator, err := NewVoteAggregator(
		ValidationRules{
			ProtocolVersion: state.ProtocolVersion,
			ChainID:         state.ChainID,
			RequireSender:   true,
		},
		state,
		validators,
		votingPower,
	)
	if err != nil {
		return err
	}

	var previous []byte
	for i, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit {
			return ErrInvalidPrecommitCertificate
		}
		if i > 0 && bytes.Compare(previous, vote.Sender) >= 0 {
			return ErrInvalidPrecommitCertificate
		}
		previous = append(previous[:0], vote.Sender...)
		if err := aggregator.AddVote(vote); err != nil {
			return fmt.Errorf("validate precommit vote: %w", err)
		}
	}

	quorum, err := aggregator.QuorumForPayload(certificate.Payload, certificate.Threshold)
	if err != nil {
		return err
	}
	if !quorum {
		return ErrPrecommitQuorumNotReached
	}
	return nil
}

func (c PrecommitCertificate) validateContext(state RoundState) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if c.ProtocolVersion != state.ProtocolVersion ||
		c.ChainID != state.ChainID ||
		c.Epoch != state.Epoch ||
		c.Height != state.Height ||
		c.Round != state.Round {
		return ErrStateContextMismatch
	}
	return nil
}

// LockProof binds a precommit quorum certificate to the proposal and round
// claimed by the lock evidence. It is the explicit handoff object for later
// round-change/timeout integration.
type LockProof struct {
	LockedRound uint64
	Proposal    []byte
	Certificate PrecommitCertificate
}

func NewLockProof(
	lockedRound uint64,
	proposal []byte,
	certificate PrecommitCertificate,
) (LockProof, error) {
	if len(proposal) == 0 || lockedRound != certificate.Round ||
		!bytes.Equal(proposal, certificate.Payload) {
		return LockProof{}, ErrInvalidLockProof
	}
	if err := certificate.Threshold.Validate(); err != nil {
		return LockProof{}, err
	}
	return LockProof{
		LockedRound: lockedRound,
		Proposal:    append([]byte(nil), proposal...),
		Certificate: PrecommitCertificate{
			ProtocolVersion: certificate.ProtocolVersion,
			ChainID:         certificate.ChainID,
			Epoch:           certificate.Epoch,
			Height:          certificate.Height,
			Round:           certificate.Round,
			Payload:         append([]byte(nil), certificate.Payload...),
			Threshold:       certificate.Threshold,
			Votes:           cloneVotes(certificate.Votes),
		},
	}, nil
}

// EncodeLockProof returns a canonical binary representation suitable for
// embedding in signed timeout evidence. It includes the complete precommit
// quorum evidence so a receiver can independently validate the proof.
func EncodeLockProof(proof LockProof) ([]byte, error) {
	if err := proof.Certificate.Threshold.Validate(); err != nil {
		return nil, err
	}
	if len(proof.Proposal) == 0 || proof.LockedRound != proof.Certificate.Round ||
		!bytes.Equal(proof.Proposal, proof.Certificate.Payload) {
		return nil, ErrInvalidLockProof
	}
	var out bytes.Buffer
	putU64(&out, proof.LockedRound)
	putU16(&out, uint16(proof.Certificate.ProtocolVersion))
	putU64(&out, uint64(proof.Certificate.ChainID))
	putU64(&out, proof.Certificate.Epoch)
	putU64(&out, uint64(proof.Certificate.Height))
	putU64(&out, proof.Certificate.Round)
	putU64(&out, proof.Certificate.Threshold.Numerator)
	putU64(&out, proof.Certificate.Threshold.Denominator)
	putBytes(&out, proof.Proposal)
	putBytes(&out, proof.Certificate.Payload)
	if uint64(len(proof.Certificate.Votes)) > uint64(^uint32(0)) {
		return nil, ErrInvalidLockProof
	}
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(proof.Certificate.Votes)))
	out.Write(count[:])
	for _, vote := range proof.Certificate.Votes {
		if vote.Type != MessageTypePrecommit || len(vote.Sender) == 0 {
			return nil, ErrInvalidLockProof
		}
		putBytes(&out, vote.Sender)
		out.WriteByte(byte(vote.Type))
		putBytes(&out, vote.Payload)
		putBytes(&out, vote.Signature)
	}
	return out.Bytes(), nil
}

// DecodeLockProof decodes and structurally validates canonical lock-proof
// evidence. Consensus validity is checked separately by ValidateLockProof.
func DecodeLockProof(encoded []byte) (LockProof, error) {
	read := func(n int) ([]byte, error) {
		if n < 0 || n > len(encoded) { return nil, ErrInvalidLockProof }
		v := encoded[:n]
		encoded = encoded[n:]
		return v, nil
	}
	readU16 := func() (uint16, error) { b, err := read(2); if err != nil { return 0, err }; return binary.BigEndian.Uint16(b), nil }
	readU32 := func() (uint32, error) { b, err := read(4); if err != nil { return 0, err }; return binary.BigEndian.Uint32(b), nil }
	readU64 := func() (uint64, error) { b, err := read(8); if err != nil { return 0, err }; return binary.BigEndian.Uint64(b), nil }
	readBytes := func() ([]byte, error) {
		n, err := readU32(); if err != nil { return nil, err }
		if uint64(n) > uint64(len(encoded)) { return nil, ErrInvalidLockProof }
		return read(int(n))
	}
	lockedRound, err := readU64(); if err != nil { return LockProof{}, err }
	version, err := readU16(); if err != nil { return LockProof{}, err }
	chainID, err := readU64(); if err != nil { return LockProof{}, err }
	epoch, err := readU64(); if err != nil { return LockProof{}, err }
	height, err := readU64(); if err != nil { return LockProof{}, err }
	round, err := readU64(); if err != nil { return LockProof{}, err }
	numerator, err := readU64(); if err != nil { return LockProof{}, err }
	denominator, err := readU64(); if err != nil { return LockProof{}, err }
	proposal, err := readBytes(); if err != nil { return LockProof{}, err }
	payload, err := readBytes(); if err != nil { return LockProof{}, err }
	count, err := readU32(); if err != nil { return LockProof{}, err }
	votes := make([]Message, 0, int(count))
	for i := uint32(0); i < count; i++ {
		sender, err := readBytes(); if err != nil { return LockProof{}, err }
		typeBytes, err := read(1); if err != nil { return LockProof{}, err }
		votePayload, err := readBytes(); if err != nil { return LockProof{}, err }
		signature, err := readBytes(); if err != nil { return LockProof{}, err }
		votes = append(votes, Message{
			ProtocolVersion: types.ProtocolVersion(version), ChainID: types.ChainID(chainID),
			Epoch: epoch, Height: types.Height(height), Round: round,
			Sender: append([]byte(nil), sender...), Type: MessageType(typeBytes[0]),
			Payload: append([]byte(nil), votePayload...), Signature: append([]byte(nil), signature...),
		})
	}
	if len(encoded) != 0 { return LockProof{}, ErrInvalidLockProof }
	certificate := PrecommitCertificate{
		ProtocolVersion: types.ProtocolVersion(version), ChainID: types.ChainID(chainID), Epoch: epoch,
		Height: types.Height(height), Round: round, Payload: append([]byte(nil), payload...),
		Threshold: QuorumThreshold{Numerator: numerator, Denominator: denominator}, Votes: votes,
	}
	return LockProof{LockedRound: lockedRound, Proposal: append([]byte(nil), proposal...), Certificate: certificate}, nil
}

func ValidateLockProof(
	proof LockProof,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
) error {
	if len(proof.Proposal) == 0 || proof.LockedRound != proof.Certificate.Round ||
		!bytes.Equal(proof.Proposal, proof.Certificate.Payload) {
		return ErrInvalidLockProof
	}
	if err := ValidatePrecommitCertificate(proof.Certificate, state, validators, votingPower); err != nil {
		return err
	}
	return nil
}
