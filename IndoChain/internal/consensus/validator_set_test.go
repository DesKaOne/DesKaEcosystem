package consensus

import (
	"errors"
	"testing"
)

func TestNewValidatorSetSortsAndClones(t *testing.T) {
	input := [][]byte{{3}, {1}, {2}}
	set, err := NewValidatorSet(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0][0] = 9
	want := [][]byte{{1}, {2}, {3}}
	for i := range want {
		if len(set.Validators[i]) != 1 || set.Validators[i][0] != want[i][0] {
			t.Fatalf("unexpected validator order: %#v", set.Validators)
		}
	}
}

func TestNewValidatorSetRejectsEmptyAndDuplicate(t *testing.T) {
	if _, err := NewValidatorSet([][]byte{{1}, {}}); !errors.Is(err, ErrEmptyValidatorID) {
		t.Fatalf("expected empty validator error, got %v", err)
	}
	if _, err := NewValidatorSet([][]byte{{1}, {1}}); !errors.Is(err, ErrDuplicateValidator) {
		t.Fatalf("expected duplicate validator error, got %v", err)
	}
}

func TestValidatorSetValidateRejectsUnsortedOrDuplicate(t *testing.T) {
	set := ValidatorSet{Validators: [][]byte{{2}, {1}}}
	if !errors.Is(set.Validate(), ErrInvalidValidatorSet) {
		t.Fatalf("expected invalid set, got %v", set.Validate())
	}
}

func TestValidatorSetContainsAndRequire(t *testing.T) {
	set, err := NewValidatorSet([][]byte{{2}, {1}})
	if err != nil {
		t.Fatal(err)
	}
	if !set.Contains([]byte{1}) {
		t.Fatal("expected validator to be present")
	}
	if set.Contains(nil) {
		t.Fatal("nil validator must not be present")
	}
	if err := set.Require([]byte{3}); !errors.Is(err, ErrValidatorNotFound) {
		t.Fatalf("expected validator-not-found, got %v", err)
	}
}

func TestValidatorSetValidationIsPure(t *testing.T) {
	set, _ := NewValidatorSet([][]byte{{1}, {2}})
	before := append([]byte(nil), set.Validators[0]...)
	if err := set.Validate(); err != nil {
		t.Fatal(err)
	}
	if set.Validators[0][0] != before[0] {
		t.Fatal("validation mutated validator set")
	}
}
