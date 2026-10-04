package state

import (
	"testing"

	"github.com/DesKaOne/DesKaEcosystem/IndoChain/internal/core/types"
)

func TestRootDeterministicAcrossInsertionOrder(t *testing.T) {
	a := New()
	a.Set(types.Address{2}, Account{Balance: 20, Nonce: 2})
	a.Set(types.Address{1}, Account{Balance: 10, Nonce: 1})

	b := New()
	b.Set(types.Address{1}, Account{Balance: 10, Nonce: 1})
	b.Set(types.Address{2}, Account{Balance: 20, Nonce: 2})

	if a.Root() != b.Root() {
		t.Fatalf("state root depends on insertion order: %s != %s", a.Root(), b.Root())
	}
}

func TestRootChangesWhenStateChanges(t *testing.T) {
	s := New()
	s.Set(types.Address{1}, Account{Balance: 10, Nonce: 0})
	first := s.Root()

	s.Set(types.Address{1}, Account{Balance: 11, Nonce: 0})
	second := s.Root()

	if first == second {
		t.Fatal("state root did not change after state mutation")
	}
}

func TestEmptyRootIsDeterministic(t *testing.T) {
	a := New()
	b := New()
	if a.Root() != b.Root() {
		t.Fatal("empty state roots differ")
	}
}
