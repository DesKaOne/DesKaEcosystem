package consensus

import "errors"

var ErrInvalidValidatorAuthority = errors.New("invalid validator authority")

// StaticValidatorAuthority is an immutable snapshot of validator public-key
// authority. Constructor input and resolver outputs are defensively copied so
// callers cannot mutate the authority through an aliased byte slice or map.
type StaticValidatorAuthority struct {
	keys map[string][]byte
}

// NewStaticValidatorAuthority creates an immutable authority snapshot.
func NewStaticValidatorAuthority(keys map[string][]byte) (StaticValidatorAuthority, error) {
	if keys == nil {
		return StaticValidatorAuthority{keys: map[string][]byte{}}, nil
	}
	cloned := make(map[string][]byte, len(keys))
	for id, key := range keys {
		if id == "" || len(key) == 0 {
			return StaticValidatorAuthority{}, ErrInvalidValidatorAuthority
		}
		cloned[id] = append([]byte(nil), key...)
	}
	return StaticValidatorAuthority{keys: cloned}, nil
}

// PublicKeyForValidator resolves a validator public key from the immutable
// authority snapshot and returns a defensive copy.
func (a StaticValidatorAuthority) PublicKeyForValidator(validatorID []byte) ([]byte, error) {
	key, ok := a.keys[string(validatorID)]
	if !ok {
		return nil, ErrConsensusAuthorityMissing
	}
	return append([]byte(nil), key...), nil
}
