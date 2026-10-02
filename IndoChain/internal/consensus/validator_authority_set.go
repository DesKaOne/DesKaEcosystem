package consensus

import (
	"bytes"
	"errors"
	"sort"
)

var (
	ErrInvalidValidatorAuthoritySet = errors.New("invalid validator authority set")
	ErrValidatorAuthorityMismatch = errors.New("validator authority mismatch")
)

// ValidatorAuthoritySet is the immutable, epoch-bound authority snapshot used
// by consensus authentication. Validator membership, public-key authority and
// epoch are bound together so a key from another validator context cannot be
// silently reused after an epoch transition.
type ValidatorAuthoritySet struct {
	Epoch uint64
	Validators ValidatorSet
	Keys map[string][]byte
}

// NewValidatorAuthoritySet constructs an immutable validator authority
// snapshot. It requires exactly one non-empty public key for every validator.
func NewValidatorAuthoritySet(epoch uint64, validators ValidatorSet, keys map[string][]byte) (ValidatorAuthoritySet, error) {
	if err := validators.Validate(); err != nil {
		return ValidatorAuthoritySet{}, err
	}
	if keys == nil {
		return ValidatorAuthoritySet{}, ErrInvalidValidatorAuthoritySet
	}
	cloned := make(map[string][]byte, len(keys))
	for _, id := range validators.Validators {
		key, ok := keys[string(id)]
		if !ok || len(key) == 0 {
			return ValidatorAuthoritySet{}, ErrValidatorAuthorityMismatch
		}
		cloned[string(id)] = append([]byte(nil), key...)
	}
	if len(cloned) != len(validators.Validators) {
		return ValidatorAuthoritySet{}, ErrValidatorAuthorityMismatch
	}
	return ValidatorAuthoritySet{
		Epoch: epoch,
		Validators: cloneValidatorSet(validators),
		Keys: cloned,
	}, nil
}

func (a ValidatorAuthoritySet) Validate() error {
	if err := a.Validators.Validate(); err != nil {
		return err
	}
	if len(a.Keys) != len(a.Validators.Validators) {
		return ErrValidatorAuthorityMismatch
	}
	for _, id := range a.Validators.Validators {
		key, ok := a.Keys[string(id)]
		if !ok || len(key) == 0 {
			return ErrValidatorAuthorityMismatch
		}
	}
	return nil
}

func (a ValidatorAuthoritySet) Contains(id []byte) bool {
	return a.Validators.Contains(id)
}

func (a ValidatorAuthoritySet) PublicKeyForValidator(id []byte) ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	key, ok := a.Keys[string(id)]
	if !ok {
		return nil, ErrConsensusAuthorityMissing
	}
	return append([]byte(nil), key...), nil
}

func (a ValidatorAuthoritySet) ValidatorSet() ValidatorSet {
	return cloneValidatorSet(a.Validators)
}

func (a ValidatorAuthoritySet) Clone() ValidatorAuthoritySet {
	cloned := ValidatorAuthoritySet{
		Epoch: a.Epoch,
		Validators: cloneValidatorSet(a.Validators),
		Keys: make(map[string][]byte, len(a.Keys)),
	}
	for id, key := range a.Keys {
		cloned.Keys[id] = append([]byte(nil), key...)
	}
	return cloned
}

func authorityDigest(a ValidatorAuthoritySet) [32]byte {
	// Keep digest construction deterministic and independent of map iteration.
	ids := append([][]byte(nil), a.Validators.Validators...)
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i], ids[j]) < 0 })
	h := newDigestWriter()
	h.WriteU64(a.Epoch)
	for _, id := range ids {
		h.WriteBytes(id)
		h.WriteBytes(a.Keys[string(id)])
	}
	return h.Sum()
}
