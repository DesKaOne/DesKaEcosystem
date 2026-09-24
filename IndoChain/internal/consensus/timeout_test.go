package consensus

import (
	"bytes"
	"errors"
	"testing"
)

func TestTimeoutCertificateRequiresQuorumAndCanonicalOrdering(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	certificate, err := NewTimeoutCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		1,
		[][]byte{[]byte("validator-b"), []byte("validator-a")},
	)
	if err != nil {
		t.Fatal(err)
	}
	if certificate.NextRound != 1 {
		t.Fatalf("unexpected next round: %d", certificate.NextRound)
	}
	if !bytes.Equal(certificate.Validators[0], []byte("validator-a")) ||
		!bytes.Equal(certificate.Validators[1], []byte("validator-b")) {
		t.Fatalf("timeout validators are not canonically ordered: %q", certificate.Validators)
	}
	if err := ValidateTimeoutCertificate(certificate, state, validators, power); err != nil {
		t.Fatal(err)
	}
}

func TestTimeoutCertificateRejectsInsufficientQuorum(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	_, err := NewTimeoutCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		1,
		[][]byte{[]byte("validator-c")},
	)
	if !errors.Is(err, ErrTimeoutQuorumNotReached) {
		t.Fatalf("expected timeout quorum error, got %v", err)
	}
}

func TestTimeoutCertificateRejectsDuplicateAndStaleRound(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	_, err := NewTimeoutCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		state.Round,
		[][]byte{[]byte("validator-a"), []byte("validator-a")},
	)
	if !errors.Is(err, ErrInvalidTimeoutRound) {
		t.Fatalf("expected invalid timeout round, got %v", err)
	}

	_, err = NewTimeoutCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		1,
		[][]byte{[]byte("validator-a"), []byte("validator-a")},
	)
	if !errors.Is(err, ErrInvalidTimeoutCertificate) {
		t.Fatalf("expected duplicate validator error, got %v", err)
	}
}

func TestValidateTimeoutCertificateRejectsNonCanonicalValidatorOrder(t *testing.T) {
	_, state, validators, power := runtimeFixture(t)
	certificate, err := NewTimeoutCertificate(
		state, validators, power,
		QuorumThreshold{Numerator: 2, Denominator: 3},
		1,
		[][]byte{[]byte("validator-a"), []byte("validator-b")},
	)
	if err != nil {
		t.Fatal(err)
	}
	certificate.Validators[0], certificate.Validators[1] = certificate.Validators[1], certificate.Validators[0]
	if err := ValidateTimeoutCertificate(certificate, state, validators, power); !errors.Is(err, ErrInvalidTimeoutCertificate) {
		t.Fatalf("expected canonical ordering error, got %v", err)
	}
}
