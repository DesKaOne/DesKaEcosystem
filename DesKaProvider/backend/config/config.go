package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type DigiFlazzConfig struct {
	Username        string
	APIKey          string
	BaseURL         string
	HTTPTimeout     time.Duration
	Endpoint        string
	BalanceEndpoint string
	PriceListEndpoint string
}

func LoadDigiFlazzConfig() (DigiFlazzConfig, error) {
	cfg := DigiFlazzConfig{
		Username:          os.Getenv("DIGIFLAZZ_USERNAME"),
		APIKey:            os.Getenv("DIGIFLAZZ_API_KEY"),
		BaseURL:           strings.TrimRight(strings.TrimSpace(os.Getenv("DIGIFLAZZ_BASE_URL")), "/"),
		Endpoint:          os.Getenv("DIGIFLAZZ_ENDPOINT"),
		BalanceEndpoint:   os.Getenv("DIGIFLAZZ_BALANCE_ENDPOINT"),
		PriceListEndpoint: os.Getenv("DIGIFLAZZ_PRICE_LIST_ENDPOINT"),
		HTTPTimeout:       15 * time.Second,
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.digiflazz.com"
	}
	if raw := strings.TrimSpace(os.Getenv("DIGIFLAZZ_HTTP_TIMEOUT")); raw != "" {
		v, err := time.ParseDuration(raw)
		if err != nil || v <= 0 { return DigiFlazzConfig{}, fmt.Errorf("invalid DIGIFLAZZ_HTTP_TIMEOUT: %q", raw) }
		cfg.HTTPTimeout = v
	}
	if cfg.Endpoint == "" { cfg.Endpoint = cfg.BaseURL + "/v1/transaction" }
	if cfg.BalanceEndpoint == "" { cfg.BalanceEndpoint = cfg.BaseURL + "/v1/cek-saldo" }
	if cfg.PriceListEndpoint == "" { cfg.PriceListEndpoint = cfg.BaseURL + "/v1/price-list" }
	if cfg.InquiryPLNEndpoint == "" { cfg.InquiryPLNEndpoint = cfg.BaseURL + "/v1/inquiry-pln" }
	if cfg.Username == "" {
		return DigiFlazzConfig{}, fmt.Errorf("DIGIFLAZZ_USERNAME is required")
	}
	if cfg.APIKey == "" {
		return DigiFlazzConfig{}, fmt.Errorf("DIGIFLAZZ_API_KEY is required")
	}
	return cfg, nil
}
