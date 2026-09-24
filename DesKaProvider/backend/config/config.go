package config

import (
	"fmt"
	"os"
)

type DigiFlazzConfig struct {
	Username string
	APIKey   string
	Endpoint string
}

func LoadDigiFlazzConfig() (DigiFlazzConfig, error) {
	cfg := DigiFlazzConfig{
		Username: os.Getenv("DIGIFLAZZ_USERNAME"),
		APIKey:   os.Getenv("DIGIFLAZZ_API_KEY"),
		Endpoint: os.Getenv("DIGIFLAZZ_ENDPOINT"),
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://api.digiflazz.com/v1/transaction"
	}
	if cfg.Username == "" {
		return DigiFlazzConfig{}, fmt.Errorf("DIGIFLAZZ_USERNAME is required")
	}
	if cfg.APIKey == "" {
		return DigiFlazzConfig{}, fmt.Errorf("DIGIFLAZZ_API_KEY is required")
	}
	return cfg, nil
}
