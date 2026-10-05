package consensus

import (
	"errors"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

var (
	ErrInvalidExecutionAuthority = errors.New("invalid execution authority")
	ErrExecutionAuthorityMissing = errors.New("execution authority missing")
)

// ExecutionAuthorityResolver is the explicit handoff from consensus identity
// to transaction-execution signature authority. The resolver is supplied by
// the execution/node layer; consensus does not invent or persist a mapping.
type ExecutionAuthorityResolver interface {
	PublicKeyForValidator(validatorID []byte) ([]byte, error)
}

// FinalizedBlockAuthorization binds a finalized block hash to the validator
// identity that must be resolved by the execution layer before commit.
type FinalizedBlockAuthorization struct {
	BlockHash    types.Hash
	Proposer     []byte
	Certificate  FinalityCertificate
}

func (a FinalizedBlockAuthorization) Validate() error {
	if a.BlockHash == (types.Hash{}) || len(a.Proposer) == 0 {
		return ErrInvalidExecutionAuthority
	}
	if len(a.Certificate.Payload) == 0 {
		return ErrInvalidExecutionAuthority
	}
	if string(a.BlockHash[:]) != string(a.Certificate.Payload) {
		return ErrInvalidExecutionAuthority
	}
	return nil
}

// ResolveProposerAuthority performs only the explicit authority handoff.
// It does not mutate state and does not establish a protocol-level registry.
func ResolveProposerAuthority(
	authorization FinalizedBlockAuthorization,
	resolver ExecutionAuthorityResolver,
) ([]byte, error) {
	if err := authorization.Validate(); err != nil {
		return nil, err
	}
	if resolver == nil {
		return nil, ErrExecutionAuthorityMissing
	}
	proposer := append([]byte(nil), authorization.Proposer...)
	publicKey, err := resolver.PublicKeyForValidator(proposer)
	if err != nil {
		return nil, err
	}
	if len(publicKey) == 0 {
		return nil, ErrExecutionAuthorityMissing
	}
	return append([]byte(nil), publicKey...), nil
}
