package payment

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidRequest = errors.New("invalid payment request")
	ErrUnsupported    = errors.New("payment capability unsupported")
)

type Status string

const (
	StatusPending Status = "pending"
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
)

type PaymentRequest struct {
	ReferenceID string
	Amount      int64
	Currency    string
	CustomerID  string
	Description string
}

type PaymentResult struct {
	ReferenceID       string
	ProviderReference string
	Status            Status
	Amount            int64
	Currency          string
	Message           string
}

type StatusRequest struct {
	ReferenceID       string
	ProviderReference string
}

type StatusResult struct {
	ReferenceID       string
	ProviderReference string
	Status            Status
	Amount            int64
	Currency          string
	Message           string
	CheckedAt         time.Time
}

// Provider is the minimum provider-neutral payment collection contract.
//
// The contract intentionally contains no provider-specific request fields,
// authentication material, HTTP status codes, webhook payloads, or protocol
// concepts. Implementations remain responsible for translating their own
// provider protocol into these neutral types.
type Provider interface {
	CreatePayment(context.Context, PaymentRequest) (PaymentResult, error)
	GetPaymentStatus(context.Context, StatusRequest) (StatusResult, error)
}

// WebhookProvider is optional. Webhook support is a separate capability and
// must not be inferred from Provider.
type WebhookProvider interface {
	HandlePaymentWebhook(context.Context, []byte) (StatusResult, error)
}

// RefundProvider is optional. Refund support is deliberately separate from
// payment creation/status and must only be registered when the provider
// contract has been explicitly verified.
type RefundProvider interface {
	RefundPayment(context.Context, string, int64, string) (StatusResult, error)
}

func ValidateRequest(req PaymentRequest) error {
	if req.ReferenceID == "" || req.Amount <= 0 || req.Currency == "" {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateStatusRequest(req StatusRequest) error {
	if req.ReferenceID == "" && req.ProviderReference == "" {
		return ErrInvalidRequest
	}
	return nil
}

func ValidateStatus(status Status) error {
	switch status {
	case StatusPending, StatusSuccess, StatusFailed:
		return nil
	default:
		return ErrInvalidRequest
	}
}
