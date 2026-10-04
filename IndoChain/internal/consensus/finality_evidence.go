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
	ErrInvalidFinalityEvidence = errors.New("invalid consensus finality evidence")
	ErrFinalityEvidenceNotReady = errors.New("consensus finality evidence not ready")
)

// BuildFinalityEvidence derives finality evidence from the authenticated
// precommit quorum without mutating runtime phase or canonical state.
func (r *ValidatorRuntime) BuildFinalityEvidence(resolver TimeoutAuthorityResolver) (FinalityCertificate, error) {
	if r == nil {
		return FinalityCertificate{}, ErrInvalidConsensusRuntime
	}
	if resolver == nil {
		return FinalityCertificate{}, ErrAuthenticatedConsensusAuthorityMissing
	}
	if r.state.Phase != PhasePrecommit {
		return FinalityCertificate{}, ErrFinalityEvidenceNotReady
	}
	if len(r.proposal) == 0 || len(r.lockedProposal) == 0 || !bytes.Equal(r.proposal, r.lockedProposal) {
		return FinalityCertificate{}, ErrConflictingLockedProposal
	}
	precommitCertificate, err := NewPrecommitCertificate(
		r.state, r.validators, r.votingPower, r.threshold,
		r.proposal, r.precommits.VotesForPayload(r.proposal),
	)
	if err != nil {
		return FinalityCertificate{}, err
	}
	if err := ValidatePrecommitCertificateWithAuthority(
		precommitCertificate, r.state, r.validators, r.votingPower, resolver,
	); err != nil {
		return FinalityCertificate{}, err
	}
	certificate, err := NewFinalityCertificate(
		r.state, r.validators, r.votingPower, r.threshold,
		r.proposal, precommitCertificate.Votes,
	)
	if err != nil {
		return FinalityCertificate{}, err
	}
	if err := r.ValidateFinalityEvidence(certificate, resolver); err != nil {
		return FinalityCertificate{}, err
	}
	return certificate, nil
}

// ValidateFinalityEvidence validates finality evidence against the current
// proposal/lock and verifies every constituent precommit signature.
// Validation is non-mutating.
func (r *ValidatorRuntime) ValidateFinalityEvidence(
	certificate FinalityCertificate,
	resolver TimeoutAuthorityResolver,
) error {
	if r == nil {
		return ErrInvalidConsensusRuntime
	}
	if resolver == nil {
		return ErrAuthenticatedConsensusAuthorityMissing
	}
	if err := ValidateFinalityCertificate(
		certificate, r.state, r.validators, r.votingPower,
	); err != nil {
		return err
	}
	if len(r.proposal) == 0 || !bytes.Equal(certificate.Payload, r.proposal) {
		return ErrInvalidFinalityEvidence
	}
	if len(r.lockedProposal) == 0 || !bytes.Equal(certificate.Payload, r.lockedProposal) {
		return ErrConflictingLockedProposal
	}
	for _, vote := range certificate.Votes {
		if vote.Type != MessageTypePrecommit {
			return ErrInvalidFinalityEvidence
		}
		if err := verifyValidatorMessageSignature(vote, resolver); err != nil {
			return err
		}
	}
	return nil
}

// EncodeFinalityCertificate returns a deterministic development encoding for
// transport. It is evidence encoding only and does not freeze block encoding.
func EncodeFinalityCertificate(certificate FinalityCertificate) ([]byte, error) {
	if err := certificate.Threshold.Validate(); err != nil {
		return nil, err
	}
	if len(certificate.Payload) == 0 || len(certificate.Votes) == 0 {
		return nil, ErrInvalidFinalityCertificate
	}
	votes := cloneVotes(certificate.Votes)
	sort.Slice(votes, func(i, j int) bool { return bytes.Compare(votes[i].Sender, votes[j].Sender) < 0 })
	var out bytes.Buffer
	putU16(&out, uint16(certificate.ProtocolVersion))
	putU64(&out, uint64(certificate.ChainID))
	putU64(&out, certificate.Epoch)
	putU64(&out, uint64(certificate.Height))
	putU64(&out, certificate.Round)
	putU64(&out, certificate.Threshold.Numerator)
	putU64(&out, certificate.Threshold.Denominator)
	putBytes(&out, certificate.Payload)
	if uint64(len(votes)) > uint64(^uint32(0)) {
		return nil, ErrInvalidFinalityCertificate
	}
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(votes)))
	out.Write(count[:])
	for _, vote := range votes {
		if vote.Type != MessageTypePrecommit || len(vote.Sender) == 0 || len(vote.Payload) == 0 || len(vote.Signature) == 0 {
			return nil, ErrInvalidFinalityCertificate
		}
		putBytes(&out, vote.Sender)
		out.WriteByte(byte(vote.Type))
		putBytes(&out, vote.Payload)
		putBytes(&out, vote.Signature)
	}
	return out.Bytes(), nil
}

// DecodeFinalityCertificate decodes only the evidence envelope. Consensus
// validity and signatures are checked separately by ValidateFinalityEvidence.
func DecodeFinalityCertificate(encoded []byte) (FinalityCertificate, error) {
	read := func(n int) ([]byte, error) {
		if n < 0 || n > len(encoded) { return nil, ErrInvalidFinalityCertificate }
		v := encoded[:n]
		encoded = encoded[n:]
		return v, nil
	}
	readU16 := func() (uint16, error) { b, err := read(2); if err != nil { return 0, err }; return binary.BigEndian.Uint16(b), nil }
	readU32 := func() (uint32, error) { b, err := read(4); if err != nil { return 0, err }; return binary.BigEndian.Uint32(b), nil }
	readU64 := func() (uint64, error) { b, err := read(8); if err != nil { return 0, err }; return binary.BigEndian.Uint64(b), nil }
	readBytes := func() ([]byte, error) {
		n, err := readU32(); if err != nil { return nil, err }
		if uint64(n) > uint64(len(encoded)) { return nil, ErrInvalidFinalityCertificate }
		return read(int(n))
	}
	version, err := readU16(); if err != nil { return FinalityCertificate{}, err }
	chainID, err := readU64(); if err != nil { return FinalityCertificate{}, err }
	epoch, err := readU64(); if err != nil { return FinalityCertificate{}, err }
	height, err := readU64(); if err != nil { return FinalityCertificate{}, err }
	round, err := readU64(); if err != nil { return FinalityCertificate{}, err }
	numerator, err := readU64(); if err != nil { return FinalityCertificate{}, err }
	denominator, err := readU64(); if err != nil { return FinalityCertificate{}, err }
	payload, err := readBytes(); if err != nil { return FinalityCertificate{}, err }
	count, err := readU32(); if err != nil { return FinalityCertificate{}, err }
	votes := make([]Message, 0, int(count))
	for i := uint32(0); i < count; i++ {
		sender, err := readBytes(); if err != nil { return FinalityCertificate{}, err }
		typeBytes, err := read(1); if err != nil { return FinalityCertificate{}, err }
		votePayload, err := readBytes(); if err != nil { return FinalityCertificate{}, err }
		signature, err := readBytes(); if err != nil { return FinalityCertificate{}, err }
		votes = append(votes, Message{
			ProtocolVersion: types.ProtocolVersion(version),
			ChainID: types.ChainID(chainID),
			Epoch: epoch,
			Height: types.Height(height),
			Round: round,
			Sender: append([]byte(nil), sender...),
			Type: MessageType(typeBytes[0]),
			Payload: append([]byte(nil), votePayload...),
			Signature: append([]byte(nil), signature...),
		})
	}
	if len(encoded) != 0 { return FinalityCertificate{}, ErrInvalidFinalityCertificate }
	return FinalityCertificate{
		ProtocolVersion: types.ProtocolVersion(version),
		ChainID: types.ChainID(chainID),
		Epoch: epoch,
		Height: types.Height(height),
		Round: round,
		Payload: append([]byte(nil), payload...),
		Threshold: QuorumThreshold{Numerator: numerator, Denominator: denominator},
		Votes: votes,
	}, nil
}

// AcceptAuthenticatedFinalityEvidence validates the transport envelope and
// then validates the embedded precommit quorum evidence. It never finalizes
// runtime state or commits canonical storage.
func (r *ValidatorRuntime) AcceptAuthenticatedFinalityEvidence(
	msg Message,
	resolver TimeoutAuthorityResolver,
) (FinalityCertificate, error) {
	if r == nil {
		return FinalityCertificate{}, ErrInvalidConsensusRuntime
	}
	if resolver == nil {
		return FinalityCertificate{}, ErrAuthenticatedConsensusAuthorityMissing
	}
	if msg.Type != MessageTypeFinalityEvidence {
		return FinalityCertificate{}, ErrInvalidFinalityEvidence
	}
	if err := ValidateConsensusMessage(msg, MessageValidationContext{
		Rules: ValidationRules{
			ProtocolVersion: r.rules.ProtocolVersion,
			ChainID: r.rules.ChainID,
			MaxPayloadSize: r.rules.MaxPayloadSize,
			RequireSender: true,
			RequireSignature: true,
		},
		State: r.state,
		Validators: r.validators,
	}); err != nil {
		return FinalityCertificate{}, err
	}
	if err := verifyValidatorMessageSignature(msg, resolver); err != nil {
		return FinalityCertificate{}, err
	}
	certificate, err := DecodeFinalityCertificate(msg.Payload)
	if err != nil {
		return FinalityCertificate{}, fmt.Errorf("decode finality evidence: %w", err)
	}
	if err := r.ValidateFinalityEvidence(certificate, resolver); err != nil {
		return FinalityCertificate{}, err
	}
	return certificate, nil
}
