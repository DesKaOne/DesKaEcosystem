package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/DesKaOne/DesKaEcosystem/DesKaProvider/runtime"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service, err := runtime.NewFromEnvironmentContext(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	if err := service.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
