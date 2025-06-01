package goscade

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sync/errgroup"
)

func CreateRunnerWithGracefulContext() (*errgroup.Group, context.Context) {
	gracefulCtx, cancelGracefulCtx := context.WithCancel(context.Background())
	runner, runnerCtx := errgroup.WithContext(gracefulCtx)

	// init call
	call := make(chan os.Signal, 1)
	signal.Notify(call, syscall.SIGINT, syscall.SIGTERM)

	// handler for closing
	runner.Go(func() error {
		select {
		case <-runnerCtx.Done():
		case <-call:
		}
		cancelGracefulCtx()
		return nil
	})

	return runner, gracefulCtx
}
