package main

import (
	"log"
	"net/http"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/internal/httpapi"
)

func main() {
	cfg := config.Load()

	mux := http.NewServeMux()
	httpapi.Register(mux)

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: mux,
	}

	log.Printf("DesKaCash backend listening on %s", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
