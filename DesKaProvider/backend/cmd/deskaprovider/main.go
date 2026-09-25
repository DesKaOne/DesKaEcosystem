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
	service, err := runtime.NewFromEnvironment(nil)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := service.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
