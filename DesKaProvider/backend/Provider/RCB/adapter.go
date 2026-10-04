// Package rcb contains the provider-neutral foundation for the RCB adapter.
//
// The concrete RCB protocol is intentionally not implemented here until its
// verified provider contract is available in the repository. This package
// therefore exposes only explicit capability declarations and rejects
// transaction operations rather than inventing provider semantics.
package rcb

import (
	"context"
	"errors"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var ErrNotImplemented = errors.New("RCB adapter operation is not implemented")

// Client is the deliberately minimal RCB adapter foundation.
//
// No credentials, endpoint, request schema, signing algorithm, provider code,
// or transaction mapping is assumed by this foundation.
type Client struct{}

func New() *Client { return &Client{} }

func (c *Client) GetProducts(context.Context, provider.ProductRequest) ([]provider.Product, error) {
	return nil, ErrNotImplemented
}

func (c *Client) Inquiry(context.Context, provider.InquiryRequest) (provider.InquiryResult, error) {
	return provider.InquiryResult{}, ErrNotImplemented
}

func (c *Client) Purchase(context.Context, provider.PurchaseRequest) (provider.PurchaseResult, error) {
	return provider.PurchaseResult{}, ErrNotImplemented
}

func (c *Client) GetStatus(context.Context, provider.StatusRequest) (provider.PurchaseStatus, error) {
	return provider.PurchaseStatus{}, ErrNotImplemented
}

func (c *Client) HandleWebhook(context.Context, provider.WebhookRequest) (provider.WebhookEvent, error) {
	return provider.WebhookEvent{}, ErrNotImplemented
}

var _ provider.PPOBProvider = (*Client)(nil)
