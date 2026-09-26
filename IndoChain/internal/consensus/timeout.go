package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidTimeoutCertificate = errors.New("invalid consensus timeout certificate")
	ErrTimeoutQuorumNotReached   = errors.New("timeout quorum not reached")
	ErrInvalidTimeoutRound       = errors.New("invalid timeout target round")
)

// TimeoutCertificate is a deterministic development evidence boundary proving
// that unique validators supported advancing from one round to a strictly
// newer round. It does not define a production timeout message, signature
// encoding, BFT algorithm, or network synchronization protocol.
type TimeoutCertificate struct {
	ProtocolVersion types.ProtocolVersion
	ChainID         types.ChainID
	Epoch           uint64
	Height          types.Height
	Round           uint64
	NextRound       uint64
	Threshold       QuorumThreshold
	Validators      [][]byte
}

// NewTimeoutCertificate constructs timeout evidence from unique validator
// identifiers and requires their voting power to reach the supplied threshold.
func NewTimeoutCertificate(
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
	threshold QuorumThreshold,
	nextRound uint64,
	senders [][]byte,
) (TimeoutCertificate, error) {
	if err := state.Validate(); err != nil {
		return TimeoutCertificate{}, err
	}
	if nextRound <= state.Round {
		return TimeoutCertificate{}, ErrInvalidTimeoutRound
	}
	if err := threshold.Validate(); err != nil {
		return TimeoutCertificate{}, err
	}
	if len(senders) == 0 {
		return TimeoutCertificate{}, ErrInvalidTimeoutCertificate
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
		return TimeoutCertificate{}, err
	}

	seen := make(map[string]struct{}, len(senders))
	var total uint64
	cloned := make([][]byte, 0, len(senders))
	for _, sender := range senders {
		if len(sender) == 0 {
			return TimeoutCertificate{}, ErrInvalidTimeoutCertificate
		}
		key := string(sender)
		if _, ok := seen[key]; ok {
			return TimeoutCertificate{}, fmt.Errorf("%w: duplicate validator", ErrInvalidTimeoutCertificate)
		}
		seen[key] = struct{}{}

		power, ok := aggregator.VotingPower.PowerOf(sender)
		if !ok {
			return TimeoutCertificate{}, ErrVoteSenderNotInVotingPower
		}
		if ^uint64(0)-total < power {
			return TimeoutCertificate{}, ErrInvalidVotingPowerSet
		}
		total += power
		cloned = append(cloned, append([]byte(nil), sender...))
	}

	totalPower, err := aggregator.VotingPower.TotalPower()
	if err != nil {
		return TimeoutCertificate{}, err
	}
	reached, err := QuorumReached(total, totalPower, threshold)
	if err != nil {
		return TimeoutCertificate{}, err
	}
	if !reached {
		return TimeoutCertificate{}, ErrTimeoutQuorumNotReached
	}

	sort.Slice(cloned, func(i, j int) bool { return bytes.Compare(cloned[i], cloned[j]) < 0 })
	return TimeoutCertificate{
		ProtocolVersion: state.ProtocolVersion,
		ChainID:         state.ChainID,
		Epoch:           state.Epoch,
		Height:          state.Height,
		Round:           state.Round,
		NextRound:       nextRound,
		Threshold:       threshold,
		Validators:      cloneByteSlices(cloned),
	}, nil
}

// ValidateTimeoutCertificate independently validates timeout evidence without
// mutating the supplied round state or validator sets.
func ValidateTimeoutCertificate(
	certificate TimeoutCertificate,
	state RoundState,
	validators ValidatorSet,
	votingPower VotingPowerSet,
) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if certificate.ProtocolVersion != state.ProtocolVersion ||
		certificate.ChainID != state.ChainID ||
		certificate.Epoch != state.Epoch ||
		certificate.Height != state.Height ||
		certificate.Round != state.Round {
		return ErrStateContextMismatch
	}
	if certificate.NextRound <= state.Round {
		return ErrInvalidTimeoutRound
	}
	if err := certificate.Threshold.Validate(); err != nil {
		return err
	}
	if len(certificate.Validators) == 0 {
		return ErrInvalidTimeoutCertificate
	}

	seen := make(map[string]struct{}, len(certificate.Validators))
	var total uint64
	for _, sender := range certificate.Validators {
		if len(sender) == 0 {
			return ErrInvalidTimeoutCertificate
		}
		key := string(sender)
		if _, ok := seen[key]; ok {
			return fmt.Errorf("%w: duplicate validator", ErrInvalidTimeoutCertificate)
		}
		seen[key] = struct{}{}
		if !validators.Contains(sender) {
			return ErrValidatorNotFound
		}
		power, ok := votingPower.PowerOf(sender)
		if !ok {
			return ErrVoteSenderNotInVotingPower
		}
		if ^uint64(0)-total < power {
			return ErrInvalidVotingPowerSet
		}
		total += power
	}
	totalPower, err := votingPower.TotalPower()
	if err != nil {
		return err
	}
	reached, err := QuorumReached(total, totalPower, certificate.Threshold)
	if err != nil {
		return err
	}
	if !reached {
		return ErrTimeoutQuorumNotReached
	}

	sorted := cloneByteSlices(certificate.Validators)
	sort.Slice(sorted, func(i, j int) bool { return bytes.Compare(sorted[i], sorted[j]) < 0 })
	if !byteSlicesEqual(sorted, certificate.Validators) {
		return ErrInvalidTimeoutCertificate
	}
	return nil
}

func cloneByteSlices(values [][]byte) [][]byte {
	cloned := make([][]byte, len(values))
	for i, value := range values {
		cloned[i] = append([]byte(nil), value...)
	}
	return cloned
}

func byteSlicesEqual(a, b [][]byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
