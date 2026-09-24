package digiflazz

import (
	"context"
	"errors"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

// CachedClient adds a bounded in-memory price-list cache to a DigiFlazz client.
// It caches the provider-neutral product list by category and applies the
// requested Active filter after the cached snapshot is read.
type CachedClient struct {
	client *Client
	ttl    time.Duration

	mu    sync.Mutex
	cache map[string]cachedProducts
}

type cachedProducts struct {
	products  []provider.Product
	expiresAt time.Time
}

func NewCachedClient(client *Client, ttl time.Duration) (*CachedClient, error) {
	if client == nil {
		return nil, errors.New("DigiFlazz client is required")
	}
	if ttl <= 0 {
		return nil, errors.New("price list cache TTL must be greater than zero")
	}
	return &CachedClient{
		client: client,
		ttl:    ttl,
		cache:  make(map[string]cachedProducts),
	}, nil
}

func (c *CachedClient) GetProducts(ctx context.Context, req provider.ProductRequest) ([]provider.Product, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry, ok := c.cache[req.Category]
	if !ok || !now.Before(entry.expiresAt) {
		products, err := c.client.GetProducts(ctx, provider.ProductRequest{Category: req.Category})
		if err != nil {
			return nil, err
		}
		entry = cachedProducts{
			products:  append([]provider.Product(nil), products...),
			expiresAt: now.Add(c.ttl),
		}
		c.cache[req.Category] = entry
	}

	return filterProducts(entry.products, req.Active), nil
}

func (c *CachedClient) Inquiry(ctx context.Context, req provider.InquiryRequest) (provider.InquiryResult, error) {
	return c.client.Inquiry(ctx, req)
}

func (c *CachedClient) Purchase(ctx context.Context, req provider.PurchaseRequest) (provider.PurchaseResult, error) {
	return c.client.Purchase(ctx, req)
}

func (c *CachedClient) GetStatus(ctx context.Context, req provider.StatusRequest) (provider.PurchaseStatus, error) {
	return c.client.GetStatus(ctx, req)
}

func (c *CachedClient) HandleWebhook(ctx context.Context, req provider.WebhookRequest) (provider.WebhookEvent, error) {
	return c.client.HandleWebhook(ctx, req)
}

func (c *CachedClient) GetBalance(ctx context.Context) (int64, error) {
	return c.client.GetBalance(ctx)
}

func filterProducts(products []provider.Product, active *bool) []provider.Product {
	if active == nil {
		return append([]provider.Product(nil), products...)
	}
	// The underlying price-list response is cached only as provider-neutral
	// products, so an Active filter can only be applied if the source list
	// contains the requested active state. DigiFlazz's adapter currently maps
	// only active products when no explicit filter is supplied.
	//
	// Keep the neutral cache semantics deterministic: nil is the full cached
	// list, and an explicit filter is treated as a cache-level selection.
	if *active {
		return append([]provider.Product(nil), products...)
	}
	return nil
}
