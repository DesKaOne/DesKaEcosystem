package config

import "os"

type Config struct {
	HTTPAddr       string
	IndoChainRPCURL string
}

func Load() Config {
	addr := os.Getenv("DESKACASH_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		HTTPAddr:        addr,
		IndoChainRPCURL: os.Getenv("INDOCHAIN_RPC_URL"),
	}
}
