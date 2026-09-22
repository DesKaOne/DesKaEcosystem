package main

import (
	"log"
	"net/http"
	"os"

	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/config"
	"github.com/DesKaOne/DesKaEcosystem/DesKaCash/backend/internal/httpapi"
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

	_ = os.Stdout
}
