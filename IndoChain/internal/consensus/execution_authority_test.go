package consensus

import (
	"errors"
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

type testAuthorityResolver struct {
	validator []byte
	publicKey []byte
	err       error
}

func (r testAuthorityResolver) PublicKeyForValidator(id []byte) ([]byte, error) {
	if r.err != nil { return nil, r.err }
	if string(id) != string(r.validator) { return nil, ErrExecutionAuthorityMissing }
	return append([]byte(nil), r.publicKey...), nil
}

type nonCloningAuthorityResolver struct {
	validator []byte
	publicKey []byte
}

func (r nonCloningAuthorityResolver) PublicKeyForValidator(id []byte) ([]byte, error) {
	if string(id) != string(r.validator) { return nil, ErrExecutionAuthorityMissing }
	return r.publicKey, nil
}

func TestResolveProposerAuthority(t *testing.T) {
	hash := types.Hash{1, 2, 3}
	authorization := FinalizedBlockAuthorization{
		BlockHash: hash,
		Proposer: []byte("validator-a"),
		Certificate: FinalityCertificate{Payload: hash[:]},
	}
	resolved, err := ResolveProposerAuthority(authorization, testAuthorityResolver{
		validator: []byte("validator-a"), publicKey: []byte("public-key"),
	})
	if err != nil { t.Fatalf("ResolveProposerAuthority() error = %v", err) }
	resolved[0] = 'X'
	if string(resolved) != "Xublic-key" {
		t.Fatal("expected returned public key to be writable clone")
	}
}

func TestResolveProposerAuthorityRequiresResolver(t *testing.T) {
	hash := types.Hash{1}
	err := FinalizedBlockAuthorization{BlockHash: hash, Proposer: []byte("v"), Certificate: FinalityCertificate{Payload: hash[:]}}.Validate()
	if err != nil { t.Fatal(err) }
	if _, err := ResolveProposerAuthority(FinalizedBlockAuthorization{BlockHash: hash, Proposer: []byte("v"), Certificate: FinalityCertificate{Payload: hash[:]}}, nil); !errors.Is(err, ErrExecutionAuthorityMissing) {
		t.Fatalf("expected missing authority resolver, got %v", err)
	}
}

func TestResolveProposerAuthorityRejectsInvalidAuthorization(t *testing.T) {
	resolver := testAuthorityResolver{
		validator: []byte("validator-a"),
		publicKey: []byte("public-key"),
	}
	cases := []struct {
		name          string
		authorization FinalizedBlockAuthorization
	}{
		{
			name:          "missing block hash",
			authorization: FinalizedBlockAuthorization{Proposer: []byte("validator-a"), Certificate: FinalityCertificate{Payload: []byte{1}}},
		},
		{
			name:          "missing proposer",
			authorization: FinalizedBlockAuthorization{BlockHash: types.Hash{1}, Certificate: FinalityCertificate{Payload: []byte{1}}},
		},
		{
			name:          "missing certificate payload",
			authorization: FinalizedBlockAuthorization{BlockHash: types.Hash{1}, Proposer: []byte("validator-a")},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ResolveProposerAuthority(tc.authorization, resolver); !errors.Is(err, ErrInvalidExecutionAuthority) {
				t.Fatalf("ResolveProposerAuthority() error = %v, want %v", err, ErrInvalidExecutionAuthority)
			}
		})
	}
}

func TestResolveProposerAuthorityValidatesBeforeResolverLookup(t *testing.T) {
	hash := types.Hash{1}
	invalid := FinalizedBlockAuthorization{
		BlockHash: hash,
		Proposer:  []byte("validator-a"),
		Certificate: FinalityCertificate{Payload: []byte{2}},
	}
	resolverErr := errors.New("resolver must not be called")
	resolver := testAuthorityResolver{err: resolverErr}
	if _, err := ResolveProposerAuthority(invalid, resolver); !errors.Is(err, ErrInvalidExecutionAuthority) {
		t.Fatalf("ResolveProposerAuthority() error = %v, want %v", err, ErrInvalidExecutionAuthority)
	}
}

func TestFinalizedBlockAuthorizationRejectsMismatchedPayload(t *testing.T) {
	hash := types.Hash{1}
	other := types.Hash{2}
	if err := (FinalizedBlockAuthorization{BlockHash: hash, Proposer: []byte("v"), Certificate: FinalityCertificate{Payload: other[:]}}).Validate(); !errors.Is(err, ErrInvalidExecutionAuthority) {
		t.Fatalf("expected invalid execution authority, got %v", err)
	}
}

func TestResolveProposerAuthorityRejectsEmptyResolvedPublicKey(t *testing.T) {
	hash := types.Hash{1}
	authorization := FinalizedBlockAuthorization{
		BlockHash: hash,
		Proposer:  []byte("validator-a"),
		Certificate: FinalityCertificate{Payload: hash[:]},
	}
	resolver := testAuthorityResolver{
		validator: []byte("validator-a"),
		publicKey: nil,
	}
	if _, err := ResolveProposerAuthority(authorization, resolver); !errors.Is(err, ErrExecutionAuthorityMissing) {
		t.Fatalf("ResolveProposerAuthority() error = %v, want %v", err, ErrExecutionAuthorityMissing)
	}
}

func TestResolveProposerAuthorityClonesResolverPublicKey(t *testing.T) {
	hash := types.Hash{1}
	authorization := FinalizedBlockAuthorization{
		BlockHash: hash,
		Proposer:  []byte("validator-a"),
		Certificate: FinalityCertificate{Payload: hash[:]},
	}
	source := []byte("public-key")
	resolved, err := ResolveProposerAuthority(authorization, nonCloningAuthorityResolver{
		validator: []byte("validator-a"),
		publicKey: source,
	})
	if err != nil { t.Fatalf("ResolveProposerAuthority() error = %v", err) }

	resolved[0] = 'X'
	if string(source) != "public-key" {
		t.Fatalf("resolver public key mutated through returned slice: %q", source)
	}
}
