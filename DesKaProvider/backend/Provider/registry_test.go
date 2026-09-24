package provider

import (
	"testing"
)

type registryTestProvider struct{}

func (registryTestProvider) GetProducts(context.Context, ProductRequest) ([]Product, error) { return nil, nil }
func (registryTestProvider) Inquiry(context.Context, InquiryRequest) (InquiryResult, error) { return InquiryResult{}, nil }
func (registryTestProvider) Purchase(context.Context, PurchaseRequest) (PurchaseResult, error) { return PurchaseResult{}, nil }
func (registryTestProvider) GetStatus(context.Context, StatusRequest) (PurchaseStatus, error) { return PurchaseStatus{}, nil }
func (registryTestProvider) HandleWebhook(context.Context, WebhookRequest) (WebhookEvent, error) { return WebhookEvent{}, nil }
