package config

import (
    "fmt"
    "os"
)

type IAKConfig struct {
    Username string
    APIKey string
    BaseURL string
    PriceListEndpoint string
    InquiryPLNEndpoint string
    TopUpEndpoint string
    StatusEndpoint string
    BalanceEndpoint string
}

func LoadIAKConfig() (IAKConfig, error) {
    cfg := IAKConfig{Username: os.Getenv("IAK_USERNAME"), APIKey: os.Getenv("IAK_API_KEY"), BaseURL: os.Getenv("IAK_BASE_URL"), PriceListEndpoint: os.Getenv("IAK_PRICE_LIST_ENDPOINT"), InquiryPLNEndpoint: os.Getenv("IAK_INQUIRY_PLN_ENDPOINT"), TopUpEndpoint: os.Getenv("IAK_TOP_UP_ENDPOINT"), StatusEndpoint: os.Getenv("IAK_STATUS_ENDPOINT"), BalanceEndpoint: os.Getenv("IAK_BALANCE_ENDPOINT")}
    if cfg.BaseURL == "" { cfg.BaseURL = "https://prepaid.iak.id" }
    if cfg.PriceListEndpoint == "" { cfg.PriceListEndpoint = cfg.BaseURL + "/api/pricelist" }
    if cfg.InquiryPLNEndpoint == "" { cfg.InquiryPLNEndpoint = cfg.BaseURL + "/api/inquiry-pln" }
    if cfg.TopUpEndpoint == "" { cfg.TopUpEndpoint = cfg.BaseURL + "/api/top-up" }
    if cfg.StatusEndpoint == "" { cfg.StatusEndpoint = cfg.BaseURL + "/api/check-status" }
    if cfg.BalanceEndpoint == "" { cfg.BalanceEndpoint = cfg.BaseURL + "/api/check-balance" }
    if cfg.Username == "" { return IAKConfig{}, fmt.Errorf("IAK_USERNAME is required") }
    if cfg.APIKey == "" { return IAKConfig{}, fmt.Errorf("IAK_API_KEY is required") }
    return cfg, nil
}