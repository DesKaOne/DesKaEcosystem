package consensus

import (
	"bytes"
	"testing"
)

func TestNewVotingPowerSetSortsAndClones(t *testing.T) {
	first := []byte("validator-b")
	second := []byte("validator-a")
	set, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: first, Power: 20},
		{ValidatorID: second, Power: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'X'
	if !bytes.Equal(set.Validators[1].ValidatorID, []byte("validator-b")) {
		t.Fatal("validator identifier was not cloned")
	}
	if got := string(set.Validators[0].ValidatorID); got != "validator-a" {
		t.Fatalf("first validator = %q", got)
	}
}

func TestNewVotingPowerSetRejectsInvalidEntries(t *testing.T) {
	cases := []struct {
		name string
		entries []ValidatorVotingPower
		wantErr error
	}{
		{"empty id", []ValidatorVotingPower{{Power: 1}}, ErrInvalidVotingPower},
		{"zero power", []ValidatorVotingPower{{ValidatorID: []byte("a")}}, ErrInvalidVotingPower},
		{"duplicate", []ValidatorVotingPower{
			{ValidatorID: []byte("a"), Power: 1},
			{ValidatorID: []byte("a"), Power: 2},
		}, ErrDuplicateVotingPowerValidator},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewVotingPowerSet(tc.entries); err != tc.wantErr {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestVotingPowerSetTotalAndLookup(t *testing.T) {
	set, err := NewVotingPowerSet([]ValidatorVotingPower{
		{ValidatorID: []byte("a"), Power: 10},
		{ValidatorID: []byte("b"), Power: 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := set.PowerOf([]byte("b")); !ok || got != 20 {
		t.Fatalf("PowerOf(b) = %d, %v", got, ok)
	}
	if got, err := set.TotalPower(); err != nil || got != 30 {
		t.Fatalf("TotalPower() = %d, %v", got, err)
	}
}

func TestQuorumReachedUsesCallerSuppliedThreshold(t *testing.T) {
	threshold := QuorumThreshold{Numerator: 2, Denominator: 3}
	cases := []struct {
		name string
		voted uint64
		total uint64
		reached bool
	}{
		{"below", 6, 10, false},
		{"exact", 7, 10, true},
		{"above", 9, 10, true},
		{"over total", 11, 10, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := QuorumReached(tc.voted, tc.total, threshold)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.reached {
				t.Fatalf("QuorumReached() = %v, want %v", got, tc.reached)
			}
		})
	}
}

func TestQuorumThresholdValidation(t *testing.T) {
	for _, threshold := range []QuorumThreshold{
		{},
		{Numerator: 0, Denominator: 3},
		{Numerator: 4, Denominator: 3},
	} {
		if err := threshold.Validate(); err != ErrInvalidQuorumThreshold {
			t.Fatalf("threshold %#v error = %v", threshold, err)
		}
	}
}
