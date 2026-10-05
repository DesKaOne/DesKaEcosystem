package config

import (
	"errors"
	"os"
	"strings"
)

const (
	defaultMidtransSnapEndpoint = "https://app.sandbox.midtrans.com/snap/v1/transactions"
	defaultMidtransAPIEndpoint  = "https://api.sandbox.midtrans.com"
)

type MidtransConfig struct {
	ServerKey     string
	SnapEndpoint  string
	APIEndpoint   string
}

func LoadMidtransConfig() (MidtransConfig, error) {
	cfg := MidtransConfig{
		ServerKey:    strings.TrimSpace(os.Getenv("MIDTRANS_SERVER_KEY")),
		SnapEndpoint: strings.TrimSpace(os.Getenv("MIDTRANS_SNAP_ENDPOINT")),
		APIEndpoint:  strings.TrimRight(strings.TrimSpace(os.Getenv("MIDTRANS_API_ENDPOINT")), "/"),
	}
	if cfg.SnapEndpoint == "" {
		cfg.SnapEndpoint = defaultMidtransSnapEndpoint
	}
	if cfg.APIEndpoint == "" {
		cfg.APIEndpoint = defaultMidtransAPIEndpoint
	}
	if cfg.ServerKey == "" {
		return MidtransConfig{}, errors.New("MIDTRANS_SERVER_KEY is required")
	}
	return cfg, nil
}
