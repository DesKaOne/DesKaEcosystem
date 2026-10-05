package consensus

import (
	"bytes"
	"errors"
	"math/big"
	"sort"
)

var (
	ErrInvalidVotingPowerSet = errors.New("invalid voting power set")
	ErrInvalidVotingPower = errors.New("invalid voting power")
	ErrDuplicateVotingPowerValidator = errors.New("duplicate voting power validator")
	ErrInvalidQuorumThreshold = errors.New("invalid quorum threshold")
)

type ValidatorVotingPower struct {
	ValidatorID []byte
	Power       uint64
}

type VotingPowerSet struct {
	Validators []ValidatorVotingPower
}

func NewVotingPowerSet(entries []ValidatorVotingPower) (VotingPowerSet, error) {
	cloned := make([]ValidatorVotingPower, len(entries))
	for i, entry := range entries {
		if len(entry.ValidatorID) == 0 || entry.Power == 0 {
			return VotingPowerSet{}, ErrInvalidVotingPower
		}
		cloned[i] = ValidatorVotingPower{
			ValidatorID: append([]byte(nil), entry.ValidatorID...),
			Power:       entry.Power,
		}
	}
	sort.Slice(cloned, func(i, j int) bool {
		return bytes.Compare(cloned[i].ValidatorID, cloned[j].ValidatorID) < 0
	})
	for i := 1; i < len(cloned); i++ {
		if bytes.Equal(cloned[i-1].ValidatorID, cloned[i].ValidatorID) {
			return VotingPowerSet{}, ErrDuplicateVotingPowerValidator
		}
	}
	return VotingPowerSet{Validators: cloned}, nil
}

func (s VotingPowerSet) Validate() error {
	for i, entry := range s.Validators {
		if len(entry.ValidatorID) == 0 || entry.Power == 0 {
			return ErrInvalidVotingPowerSet
		}
		if i > 0 && bytes.Compare(s.Validators[i-1].ValidatorID, entry.ValidatorID) >= 0 {
			return ErrInvalidVotingPowerSet
		}
	}
	return nil
}

func (s VotingPowerSet) PowerOf(id []byte) (uint64, bool) {
	if len(id) == 0 {
		return 0, false
	}
	for _, entry := range s.Validators {
		if bytes.Equal(entry.ValidatorID, id) {
			return entry.Power, true
		}
	}
	return 0, false
}

func (s VotingPowerSet) TotalPower() (uint64, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	var total uint64
	for _, entry := range s.Validators {
		if ^uint64(0)-total < entry.Power {
			return 0, ErrInvalidVotingPowerSet
		}
		total += entry.Power
	}
	return total, nil
}

type QuorumThreshold struct {
	Numerator   uint64
	Denominator uint64
}

func (q QuorumThreshold) Validate() error {
	if q.Numerator == 0 || q.Denominator == 0 || q.Numerator > q.Denominator {
		return ErrInvalidQuorumThreshold
	}
	return nil
}

func QuorumReached(votedPower, totalPower uint64, threshold QuorumThreshold) (bool, error) {
	if err := threshold.Validate(); err != nil {
		return false, err
	}
	if votedPower > totalPower {
		return false, nil
	}

	left := new(big.Int).Mul(new(big.Int).SetUint64(votedPower), new(big.Int).SetUint64(threshold.Denominator))
	right := new(big.Int).Mul(new(big.Int).SetUint64(totalPower), new(big.Int).SetUint64(threshold.Numerator))
	return left.Cmp(right) >= 0, nil
}
