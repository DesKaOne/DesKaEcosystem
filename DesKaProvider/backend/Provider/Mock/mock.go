package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

var ErrProductNotFound = errors.New("mock product not found")
var ErrTransactionNotFound = errors.New("mock transaction not found")

type Config struct {
	Products       []provider.Product
	PurchaseStatus provider.TransactionStatus
	ProviderCode   string
	Message        string
	Price          int64
}

type Provider struct {
	mu           sync.RWMutex
	products     map[string]provider.Product
	status       provider.TransactionStatus
	providerCode string
	message      string
	price        int64
	purchases    map[string]provider.PurchaseResult
}

func New(cfg Config) *Provider {
	products := make(map[string]provider.Product, len(cfg.Products))
	for _, product := range cfg.Products {
		products[product.Code] = product
	}

	status := cfg.PurchaseStatus
	if status == "" {
		status = provider.StatusSuccess
	}
	message := cfg.Message
	if message == "" {
		message = "mock transaction accepted"
	}

	return &Provider{
		products:     products,
		status:       status,
		providerCode: cfg.ProviderCode,
		message:      message,
		price:        cfg.Price,
		purchases:    make(map[string]provider.PurchaseResult),
	}
}

func (p *Provider) GetProducts(ctx context.Context, req provider.ProductRequest) ([]provider.Product, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	products := make([]provider.Product, 0, len(p.products))
	for _, product := range p.products {
		products = append(products, product)
	}
	return products, nil
}

func (p *Provider) Inquiry(ctx context.Context, req provider.InquiryRequest) (provider.InquiryResult, error) {
	if err := ctx.Err(); err != nil {
		return provider.InquiryResult{}, err
	}

	p.mu.RLock()
	_, ok := p.products[req.ProductCode]
	p.mu.RUnlock()
	if !ok {
		return provider.InquiryResult{
			Status:       provider.StatusFailed,
			ProviderCode: "PRODUCT_NOT_FOUND",
			Message:      ErrProductNotFound.Error(),
		}, nil
	}

	return provider.InquiryResult{
		Status:       provider.StatusSuccess,
		ProviderCode: p.providerCode,
		Message:      p.message,
	}, nil
}

func (p *Provider) Purchase(ctx context.Context, req provider.PurchaseRequest) (provider.PurchaseResult, error) {
	if err := ctx.Err(); err != nil {
		return provider.PurchaseResult{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.products[req.ProductCode]; !ok {
		return provider.PurchaseResult{}, fmt.Errorf("%w: %s", ErrProductNotFound, req.ProductCode)
	}

	result := provider.PurchaseResult{
		ReferenceID:  req.ReferenceID,
		CustomerNo:   req.CustomerNo,
		ProductCode:  req.ProductCode,
		Status:       p.status,
		ProviderCode: p.providerCode,
		Message:      p.message,
		Price:        p.price,
	}
	p.purchases[req.ReferenceID] = result
	return result, nil
}

func (p *Provider) GetStatus(ctx context.Context, req provider.StatusRequest) (provider.PurchaseStatus, error) {
	if err := ctx.Err(); err != nil {
		return provider.PurchaseStatus{}, err
	}

	p.mu.RLock()
	result, ok := p.purchases[req.ReferenceID]
	p.mu.RUnlock()
	if !ok {
		return provider.PurchaseStatus{}, fmt.Errorf("%w: %s", ErrTransactionNotFound, req.ReferenceID)
	}

	return provider.PurchaseStatus{
		ReferenceID:  result.ReferenceID,
		CustomerNo:   result.CustomerNo,
		ProductCode:  result.ProductCode,
		Status:       result.Status,
		ProviderCode: result.ProviderCode,
		Message:      result.Message,
		Price:        result.Price,
	}, nil
}

func (p *Provider) HandleWebhook(ctx context.Context, req provider.WebhookRequest) (provider.WebhookEvent, error) {
	if err := ctx.Err(); err != nil {
		return provider.WebhookEvent{}, err
	}

	var event provider.WebhookEvent
	if err := json.Unmarshal(req.Body, &event); err != nil {
		return provider.WebhookEvent{}, err
	}
	return event, nil
}
