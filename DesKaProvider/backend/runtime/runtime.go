package runtime

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	provider "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/catalog"
	digiflazz "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/DigiFlazz"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/routing"
)

const (
	defaultStorePath             = "data/operational-snapshots.json"
	defaultTransactionStorePath  = "data/provider-transactions.json"
	defaultSyncInterval          = 30 * time.Second
	defaultFailureThreshold      = 3
	defaultCurrency              = "IDR"
	defaultPriceListCacheTTL     = 15 * time.Minute
	defaultCatalogStorePath      = "data/product-catalog.json"
	defaultCatalogSyncInterval   = 15 * time.Minute
)

type Config struct {
	StorePath             string
	TransactionStorePath  string
	SyncInterval          time.Duration
	FailureThreshold      int
	Currency              string
	CatalogStorePath      string
	CatalogSyncInterval   time.Duration
}

type Service struct {
	syncService    *operational.SyncService
	purchaseService *routing.Service
	catalogSync    *catalog.SyncService
	interval       time.Duration
	catalogInterval time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		StorePath:            os.Getenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH"),
		TransactionStorePath: os.Getenv("DESKAPROVIDER_TRANSACTION_STORE_PATH"),
		SyncInterval:         defaultSyncInterval,
		FailureThreshold:     defaultFailureThreshold,
		Currency:             os.Getenv("DESKAPROVIDER_OPERATIONAL_CURRENCY"),
		CatalogStorePath:     os.Getenv("DESKAPROVIDER_CATALOG_STORE_PATH"),
		CatalogSyncInterval:  defaultCatalogSyncInterval,
	}
	if cfg.StorePath == "" {
		cfg.StorePath = defaultStorePath
	}
	if cfg.TransactionStorePath == "" {
		cfg.TransactionStorePath = defaultTransactionStorePath
	}
	if cfg.Currency == "" {
		cfg.Currency = defaultCurrency
	}
	if cfg.CatalogStorePath == "" {
		cfg.CatalogStorePath = defaultCatalogStorePath
	}
	if raw := os.Getenv("DESKAPROVIDER_CATALOG_SYNC_INTERVAL"); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil || interval <= 0 {
			return Config{}, fmt.Errorf("invalid DESKAPROVIDER_CATALOG_SYNC_INTERVAL: %q", raw)
		}
		cfg.CatalogSyncInterval = interval
	}

	if raw := os.Getenv("DESKAPROVIDER_BALANCE_SYNC_INTERVAL"); raw != "" {
		interval, err := time.ParseDuration(raw)
		if err != nil || interval <= 0 {
			return Config{}, fmt.Errorf("invalid DESKAPROVIDER_BALANCE_SYNC_INTERVAL: %q", raw)
		}
		cfg.SyncInterval = interval
	}
	if raw := os.Getenv("DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD"); raw != "" {
		threshold, err := strconv.Atoi(raw)
		if err != nil || threshold < 1 {
			return Config{}, fmt.Errorf("invalid DESKAPROVIDER_BALANCE_FAILURE_THRESHOLD: %q", raw)
		}
		cfg.FailureThreshold = threshold
	}
	return cfg, nil
}

func NewFromEnvironment(httpClient *http.Client) (*Service, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	digiCfg, err := config.LoadDigiFlazzConfig()
	if err != nil {
		return nil, err
	}
	client, err := digiflazz.New(digiCfg, httpClient)
	if err != nil {
		return nil, err
	}
	cachedClient, err := digiflazz.NewCachedClient(client, defaultPriceListCacheTTL)
	if err != nil {
		return nil, err
	}
	registry := provider.NewRegistry()
	if err := registry.Register("digiflazz", cachedClient); err != nil {
		return nil, err
	}
	store, err := operational.NewJSONFileStore(cfg.StorePath)
	if err != nil {
		return nil, err
	}
	syncService, err := operational.NewSyncService(registry, store, cfg.Currency, cfg.FailureThreshold)
	if err != nil {
		return nil, err
	}
	catalogStore, err := catalog.NewJSONFileStore(cfg.CatalogStorePath)
	if err != nil {
		return nil, err
	}
	catalogSync, err := catalog.NewSyncService(registry, catalogStore)
	if err != nil {
		return nil, err
	}
	transactionStore, err := routing.NewJSONFileTransactionStore(cfg.TransactionStorePath)
	if err != nil {
		return nil, err
	}
	router, err := routing.NewWithCatalog(registry, store, nil, catalogStore)
	if err != nil {
		return nil, err
	}
	purchaseService, err := routing.NewServiceWithStore(router, transactionStore)
	if err != nil {
		return nil, err
	}
	return &Service{syncService: syncService, purchaseService: purchaseService, catalogSync: catalogSync, interval: cfg.SyncInterval, catalogInterval: cfg.CatalogSyncInterval}, nil
}

func New(syncService *operational.SyncService, interval time.Duration) (*Service, error) {
	if syncService == nil {
		return nil, errors.New("sync service is required")
	}
	if interval <= 0 {
		return nil, errors.New("sync interval must be greater than zero")
	}
	return &Service{syncService: syncService, interval: interval}, nil
}

func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is required")
	}
	if s.catalogSync == nil {
		return s.syncService.Run(ctx, s.interval)
	}
	_ = s.catalogSync.SyncAll(ctx)
	ticker := time.NewTicker(s.catalogInterval)
	defer ticker.Stop()
	go func() { _ = s.syncService.Run(ctx, s.interval) }()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_ = s.catalogSync.SyncAll(ctx)
		}
	}
}

func (s *Service) PurchaseService() *routing.Service {
	if s == nil {
		return nil
	}
	return s.purchaseService
}
