package main

import (
	"context"
	"errors"

	"github.com/ognick/goscade"
	"github.com/ognick/goscade/example/internal/api"
	"github.com/ognick/goscade/example/internal/usecase"
	"github.com/ognick/goscade/example/pkg"
)

const addr = "127.0.0.1:8080"

func main() {
	log := pkg.NewLogger(pkg.LoggerCfg{
		Level:         "info",
		Development:   true,
		DisableCaller: false,
		DisableJson:   true,
	})

	// Create lifecycle with signal handling enabled
	lc := goscade.NewLifecycle(log, goscade.WithShutdownHook())
	lc.Register(pkg.NewServer(
		addr,
		api.NewHandler(
			log,
			usecase.NewUsecase(log),
		),
	))

	// Run lifecycle with blocking behavior and readiness callback
	// The lifecycle will block until shutdown and call the callback when ready
	err := goscade.Run(context.Background(), lc, func() {
		log.Infof("HTTP started on http://%s", addr)
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("%v", err)
	}
	log.Info("Application gracefully finished")
}
