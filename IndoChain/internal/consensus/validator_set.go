package consensus

import (
	"bytes"
	"errors"
	"sort"
)

var (
	ErrInvalidValidatorSet = errors.New("invalid validator set")
	ErrValidatorNotFound = errors.New("validator not found")
	ErrDuplicateValidator = errors.New("duplicate validator")
	ErrEmptyValidatorID = errors.New("empty validator id")
)

// ValidatorSet is the development consensus membership boundary.
//
// It intentionally models membership only. Stake, voting power, activation
// epochs, delegation, slashing, and validator registration transactions remain
// outside this milestone.
type ValidatorSet struct {
	Validators [][]byte
}

func NewValidatorSet(validators [][]byte) (ValidatorSet, error) {
	cloned := make([][]byte, len(validators))
	for i, id := range validators {
		if len(id) == 0 {
			return ValidatorSet{}, ErrEmptyValidatorID
		}
		cloned[i] = append([]byte(nil), id...)
	}
	sort.Slice(cloned, func(i, j int) bool { return bytes.Compare(cloned[i], cloned[j]) < 0 })
	for i := 1; i < len(cloned); i++ {
		if bytes.Equal(cloned[i-1], cloned[i]) {
			return ValidatorSet{}, ErrDuplicateValidator
		}
	}
	return ValidatorSet{Validators: cloned}, nil
}

func (s ValidatorSet) Validate() error {
	for i, id := range s.Validators {
		if len(id) == 0 {
			return ErrInvalidValidatorSet
		}
		if i > 0 && bytes.Compare(s.Validators[i-1], id) >= 0 {
			return ErrInvalidValidatorSet
		}
	}
	return nil
}

func (s ValidatorSet) Contains(id []byte) bool {
	if len(id) == 0 {
		return false
	}
	for _, candidate := range s.Validators {
		if bytes.Equal(candidate, id) {
			return true
		}
	}
	return false
}

func (s ValidatorSet) Require(id []byte) error {
	if !s.Contains(id) {
		return ErrValidatorNotFound
	}
	return nil
}
