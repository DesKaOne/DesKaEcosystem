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
	digiflazz "github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/DigiFlazz"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/Provider/operational"
	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/config"
)

const (
	defaultStorePath       = "data/operational-snapshots.json"
	defaultSyncInterval    = 30 * time.Second
	defaultFailureThreshold = 3
	defaultCurrency        = "IDR"
)

type Config struct {
	StorePath        string
	SyncInterval     time.Duration
	FailureThreshold int
	Currency         string
}

type Service struct {
	syncService *operational.SyncService
	interval    time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		StorePath:        os.Getenv("DESKAPROVIDER_OPERATIONAL_STORE_PATH"),
		SyncInterval:     defaultSyncInterval,
		FailureThreshold: defaultFailureThreshold,
		Currency:         os.Getenv("DESKAPROVIDER_OPERATIONAL_CURRENCY"),
	}
	if cfg.StorePath == "" {
		cfg.StorePath = defaultStorePath
	}
	if cfg.Currency == "" {
		cfg.Currency = defaultCurrency
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
	registry := provider.NewRegistry()
	if err := registry.Register("digiflazz", client); err != nil {
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
	return &Service{syncService: syncService, interval: cfg.SyncInterval}, nil
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
	return s.syncService.Run(ctx, s.interval)
}
