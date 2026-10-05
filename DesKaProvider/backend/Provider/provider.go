package provider

import (
	"context"
	"errors"
)

var ErrUnsupportedOperation = errors.New("provider operation is not supported")

type TransactionStatus string

const (
	StatusSuccess TransactionStatus = "success"
	StatusPending TransactionStatus = "pending"
	StatusFailed  TransactionStatus = "failed"
)

type Product struct {
	Code string
	Name string
}

type ProductRequest struct {
	Category string
	Active   *bool
}

type InquiryRequest struct {
	ProductCode string
	CustomerNo  string
	ReferenceID string
}

type InquiryResult struct {
	Status       TransactionStatus
	ProviderCode string
	Message      string
}

type PurchaseRequest struct {
	ProductCode string
	CustomerNo  string
	ReferenceID string
	Testing     bool
}

type PurchaseResult struct {
	ReferenceID  string
	CustomerNo   string
	ProductCode  string
	Status       TransactionStatus
	ProviderCode string
	Message      string
	SerialNumber string
	Price        int64
}

type StatusRequest struct {
	ProductCode string
	CustomerNo  string
	ReferenceID string
}

type PurchaseStatus struct {
	ReferenceID  string
	CustomerNo   string
	ProductCode  string
	Status       TransactionStatus
	ProviderCode string
	Message      string
	SerialNumber string
	Price        int64
}

type WebhookRequest struct {
	Body            []byte
	Event           string
	Signature       string
	SignatureSecret string
	UserAgent       string
}

type WebhookEvent struct {
	ReferenceID  string
	CustomerNo   string
	ProductCode  string
	Status       TransactionStatus
	ProviderCode string
	Message       string
	SerialNumber string
	Price        int64
}

type PPOBProvider interface {
	GetProducts(context.Context, ProductRequest) ([]Product, error)
	Inquiry(context.Context, InquiryRequest) (InquiryResult, error)
	Purchase(context.Context, PurchaseRequest) (PurchaseResult, error)
	GetStatus(context.Context, StatusRequest) (PurchaseStatus, error)
	HandleWebhook(context.Context, WebhookRequest) (WebhookEvent, error)
}

// BalanceProvider is an optional capability implemented by providers that
// expose an operational account balance endpoint.
type BalanceProvider interface {
	GetBalance(context.Context) (int64, error)
}
