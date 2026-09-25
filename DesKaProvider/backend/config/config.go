package config

import (
	"fmt"
	"os"
)

type DigiFlazzConfig struct {
	Username        string
	APIKey          string
	Endpoint        string
	BalanceEndpoint string
	PriceListEndpoint string
	InquiryPLNEndpoint string
}

func LoadDigiFlazzConfig() (DigiFlazzConfig, error) {
	cfg := DigiFlazzConfig{
		Username:          os.Getenv("DIGIFLAZZ_USERNAME"),
		APIKey:            os.Getenv("DIGIFLAZZ_API_KEY"),
		Endpoint:          os.Getenv("DIGIFLAZZ_ENDPOINT"),
		BalanceEndpoint:   os.Getenv("DIGIFLAZZ_BALANCE_ENDPOINT"),
		PriceListEndpoint: os.Getenv("DIGIFLAZZ_PRICE_LIST_ENDPOINT"),
		InquiryPLNEndpoint: os.Getenv("DIGIFLAZZ_INQUIRY_PLN_ENDPOINT"),
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://api.digiflazz.com/v1/transaction"
	}
	if cfg.BalanceEndpoint == "" {
		cfg.BalanceEndpoint = "https://api.digiflazz.com/v1/cek-saldo"
	}
	if cfg.PriceListEndpoint == "" {
		cfg.PriceListEndpoint = "https://api.digiflazz.com/v1/price-list"
	}
	if cfg.InquiryPLNEndpoint == "" {
		cfg.InquiryPLNEndpoint = "https://api.digiflazz.com/v1/inquiry-pln"
	}
	if cfg.Username == "" {
		return DigiFlazzConfig{}, fmt.Errorf("DIGIFLAZZ_USERNAME is required")
	}
	if cfg.APIKey == "" {
		return DigiFlazzConfig{}, fmt.Errorf("DIGIFLAZZ_API_KEY is required")
	}
	return cfg, nil
}
