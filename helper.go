package goscade

import (
	"context"
	"errors"
	"fmt"
)

// Register is a convenience function that registers a component with the lifecycle
// manager and returns the same component. This allows for fluent-style registration
// and makes it easier to chain component creation and registration.
//
// Example:
//
//	server := Register(lc, NewServer(handler))
//	db := Register(lc, NewDatabase(config))
//	cache := Register(lc, NewCache(), db) // explicit dependency
//
// This is equivalent to:
//
//	server := NewServer(handler)
//	lc.Register(server)
//	db := NewDatabase(config)
//	lc.Register(db)
//	cache := NewCache()
//	lc.Register(cache, db) // explicit dependency
func Register[T Component](lc Lifecycle, component T, implicitDeps ...Component) T {
	lc.Register(component, implicitDeps...)
	return component
}

// Run executes a single component with blocking behavior and readiness callback.
// This is a convenience function for running individual components outside of
// a lifecycle manager. The method blocks until the component is ready or fails.
//
// Parameters:
//   - ctx: Context for cancellation
//   - comp: Component to run
//   - ready: Callback function called when component is ready
//
// Returns:
//   - error: Any error that occurred during component execution
//
// Example:
//
//	err := goscade.Run(ctx, &MyComponent{}, func() {
//		log.Info("Component is ready")
//	})
func Run(ctx context.Context, comp Component, ready func()) error {
	probe, cancelProbe := context.WithCancelCause(ctx)
	res := make(chan error)
	go func() {
		res <- comp.Run(ctx, func(err error) {
			cancelProbe(err)
		})
	}()

	<-probe.Done()
	if err := context.Cause(probe); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("readiness probe failed: %w", err)
	}

	ready()
	if err := <-res; err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("lifecycle run failed: %w", err)
	}

	return nil
}
