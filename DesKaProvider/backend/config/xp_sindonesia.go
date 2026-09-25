package config

import (
	"fmt"
	"os"
)

type XPSindonesiaConfig struct {
	ID          string
	Key         string
	API         string
	SaldoEndpoint string
	HargaEndpoint string
	DaftarHargaEndpoint string
	OrderEndpoint string
}

func LoadXPSindonesiaConfig() (XPSindonesiaConfig, error) {
	cfg := XPSindonesiaConfig{
		ID: os.Getenv("XP_SINDONESIA_ID"),
		Key: os.Getenv("XP_SINDONESIA_KEY"),
		API: os.Getenv("XP_SINDONESIA_API"),
		SaldoEndpoint: os.Getenv("XP_SINDONESIA_SALDO_ENDPOINT"),
		HargaEndpoint: os.Getenv("XP_SINDONESIA_HARGA_ENDPOINT"),
		DaftarHargaEndpoint: os.Getenv("XP_SINDONESIA_DAFTAR_HARGA_ENDPOINT"),
		OrderEndpoint: os.Getenv("XP_SINDONESIA_ORDER_ENDPOINT"),
	}
	if cfg.SaldoEndpoint == "" { cfg.SaldoEndpoint = "https://xp.sindonesia.net/api/saldo.php" }
	if cfg.HargaEndpoint == "" { cfg.HargaEndpoint = "https://xp.sindonesia.net/api/harga.php" }
	if cfg.DaftarHargaEndpoint == "" { cfg.DaftarHargaEndpoint = "https://xp.sindonesia.net/api/daftar_harga.php" }
	if cfg.OrderEndpoint == "" { cfg.OrderEndpoint = "https://xp.sindonesia.net/api/order.php" }
	if cfg.ID == "" { return XPSindonesiaConfig{}, fmt.Errorf("XP_SINDONESIA_ID is required") }
	if cfg.Key == "" { return XPSindonesiaConfig{}, fmt.Errorf("XP_SINDONESIA_KEY is required") }
	if cfg.API == "" { return XPSindonesiaConfig{}, fmt.Errorf("XP_SINDONESIA_API is required") }
	return cfg, nil
}
