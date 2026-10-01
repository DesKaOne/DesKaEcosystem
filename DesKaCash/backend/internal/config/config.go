package config

import "os"

type Config struct {
	HTTPAddr          string
	DesKaProviderURL  string
	DesKaProviderAPIKey string
}

func Load() Config {
	addr := os.Getenv("DESKACASH_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		HTTPAddr:           addr,
		DesKaProviderURL:   os.Getenv("DESKAPROVIDER_URL"),
		DesKaProviderAPIKey: os.Getenv("DESKAPROVIDER_API_KEY"),
	}
}
