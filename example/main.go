package main

import (
	"context"
	"errors"

	"github.com/ognick/goscade"
	"github.com/ognick/goscade/example/internal/api"
	"github.com/ognick/goscade/example/internal/usecase"

	"github.com/ognick/goscade/example/pkg"
)

func main() {
	log := pkg.NewLogger(pkg.LoggerCfg{
		Level:         "info",
		Development:   true,
		DisableCaller: false,
		DisableJson:   true,
	})

	lc := goscade.NewLifecycle(log)
	lc.Register(pkg.NewServer(
		api.NewHandler(
			log,
			usecase.NewUsecase(log),
		),
	))

	waitGracefulShutdown := lc.Run(context.Background(), func(err error) {
		if err != nil {
			log.Errorf("readiness probe failed: %v", err)
		} else {
			log.Info("Server started on http://127.0.0.1:8080")
		}
	})

	// Awaiting graceful shutdown
	if err := waitGracefulShutdown(); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("%v", err)
	}

	log.Info("Application gracefully finished")
}
