package payment

import (
	"context"
	"errors"
	"testing"
)

func TestValidatePaymentRequest(t *testing.T) {
	valid := PaymentRequest{ReferenceID: "ref-1", Amount: 10000, Currency: "IDR"}
	if err := ValidateRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	cases := []PaymentRequest{
		{Amount: 10000, Currency: "IDR"},
		{ReferenceID: "ref-1", Currency: "IDR"},
		{ReferenceID: "ref-1", Amount: 10000},
	}
	for _, tc := range cases {
		if err := ValidateRequest(tc); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest, got %v", err)
		}
	}
}

func TestValidateStatusRequest(t *testing.T) {
	if err := ValidateStatusRequest(StatusRequest{ReferenceID: "ref-1"}); err != nil {
		t.Fatalf("reference ID should be accepted: %v", err)
	}
	if err := ValidateStatusRequest(StatusRequest{ProviderReference: "provider-ref-1"}); err != nil {
		t.Fatalf("provider reference should be accepted: %v", err)
	}
	if err := ValidateStatusRequest(StatusRequest{}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestValidateStatus(t *testing.T) {
	for _, status := range []Status{StatusPending, StatusSuccess, StatusFailed} {
		if err := ValidateStatus(status); err != nil {
			t.Fatalf("status %q rejected: %v", status, err)
		}
	}
	if err := ValidateStatus(Status("unknown")); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid status error, got %v", err)
	}
}

func TestPaymentOnlyProviderSatisfiesMinimumContract(t *testing.T) {
	var _ Provider = paymentOnlyProvider{}
}

type paymentOnlyProvider struct{}

func (paymentOnlyProvider) CreatePayment(_ context.Context, req PaymentRequest) (PaymentResult, error) {
	return PaymentResult{ReferenceID: req.ReferenceID, Amount: req.Amount, Currency: req.Currency, Status: StatusPending}, nil
}

func (paymentOnlyProvider) GetPaymentStatus(_ context.Context, req StatusRequest) (StatusResult, error) {
	return StatusResult{ReferenceID: req.ReferenceID, ProviderReference: req.ProviderReference, Status: StatusPending}, nil
}
