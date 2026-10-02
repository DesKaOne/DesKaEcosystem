package consensus

import (
	"bytes"
	"errors"
	"testing"
)

func TestValidatorAuthoritySetBindsEpochMembershipAndKeys(t *testing.T) {
	ids := [][]byte{[]byte("validator-a"), []byte("validator-b")}
	validators, err := NewValidatorSet(ids)
	if err != nil { t.Fatal(err) }
	keys := map[string][]byte{"validator-a": []byte("key-a"), "validator-b": []byte("key-b")}
	set, err := NewValidatorAuthoritySet(7, validators, keys)
	if err != nil { t.Fatal(err) }
	if err := set.Validate(); err != nil { t.Fatal(err) }
	got, err := set.PublicKeyForValidator([]byte("validator-a"))
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(got, []byte("key-a")) { t.Fatalf("key=%q", got) }
	got[0] = 'X'
	got2, _ := set.PublicKeyForValidator([]byte("validator-a"))
	if !bytes.Equal(got2, []byte("key-a")) { t.Fatal("resolver leaked authority alias") }
}

func TestValidatorAuthoritySetRejectsMissingOrExtraAuthority(t *testing.T) {
	validators, err := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	if err != nil { t.Fatal(err) }
	if _, err := NewValidatorAuthoritySet(1, validators, map[string][]byte{"validator-a": []byte("key-a")}); !errors.Is(err, ErrValidatorAuthorityMismatch) {
		t.Fatalf("missing key error=%v", err)
	}
	if _, err := NewValidatorAuthoritySet(1, validators, map[string][]byte{"validator-a": []byte("key-a"), "validator-b": []byte("key-b"), "validator-c": []byte("key-c")}); !errors.Is(err, ErrValidatorAuthorityMismatch) {
		t.Fatalf("extra key error=%v", err)
	}
}

func TestValidatorAuthoritySetDigestIsDeterministicAndEpochBound(t *testing.T) {
	a, _ := NewValidatorSet([][]byte{[]byte("validator-a"), []byte("validator-b")})
	first, err := NewValidatorAuthoritySet(7, a, map[string][]byte{"validator-a": []byte("key-a"), "validator-b": []byte("key-b")})
	if err != nil { t.Fatal(err) }
	second, err := NewValidatorAuthoritySet(7, a, map[string][]byte{"validator-b": []byte("key-b"), "validator-a": []byte("key-a")})
	if err != nil { t.Fatal(err) }
	if authorityDigest(first) != authorityDigest(second) { t.Fatal("authority digest depends on map order") }
	third, err := NewValidatorAuthoritySet(8, a, map[string][]byte{"validator-a": []byte("key-a"), "validator-b": []byte("key-b")})
	if err != nil { t.Fatal(err) }
	if authorityDigest(first) == authorityDigest(third) { t.Fatal("epoch change must alter authority digest") }
}
