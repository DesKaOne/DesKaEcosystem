package digiflazz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
)

// CachedClient adds a bounded in-memory price-list cache to a DigiFlazz client.
// The cache key includes the neutral category and active filter so the
// provider-specific active-state mapping remains inside the DigiFlazz client.
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

	key := cacheKey(req)
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry, ok := c.cache[key]
	if !ok || !now.Before(entry.expiresAt) {
		products, err := c.client.GetProducts(ctx, req)
		if err != nil {
			return nil, err
		}
		entry = cachedProducts{
			products:  append([]provider.Product(nil), products...),
			expiresAt: now.Add(c.ttl),
		}
		c.cache[key] = entry
	}

	return append([]provider.Product(nil), entry.products...), nil
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

func cacheKey(req provider.ProductRequest) string {
	active := "nil"
	if req.Active != nil {
		active = fmt.Sprintf("%t", *req.Active)
	}
	return req.Category + "\x00" + active
}
