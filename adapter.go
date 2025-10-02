package goscade

import (
	"context"
	"reflect"
)

// runFn defines a function type for running a delegate component.
// It takes a context, the delegate instance, and a readiness probe function.
type runFn[T any] func(ctx context.Context, delegate T, readinessProbe func(cause error)) error

// delegateNameProvider is an interface for components that can provide
// a custom name for logging and debugging purposes.
type delegateNameProvider interface {
	delegateName() string
}

// adapter wraps a delegate component and provides a way to run it
// with custom logic while maintaining the Component interface.
type adapter[T any] struct {
	delegate T
	run      runFn[T]
}

// NewAdapter creates a new adapter that wraps a delegate component.
// This is useful when you need to add custom logic around an existing
// component without modifying its implementation.
//
// Example:
//
//	server := &http.Server{Addr: ":8080"}
//	adapter := NewAdapter(server, func(ctx context.Context, srv *http.Server, probe func(error)) error {
//	    // Custom startup logic
//	    probe(nil) // Signal readiness
//	    return srv.ListenAndServe()
//	})
func NewAdapter[T any](delegate T, run runFn[T]) Component {
	return &adapter[T]{
		delegate: delegate,
		run:      run,
	}
}

// delegateName returns the string representation of the delegate's type.
// This is used for logging and debugging purposes.
func (a *adapter[T]) delegateName() string {
	return reflect.TypeOf(a.delegate).String()
}

// Run executes the adapter's run function with the provided context and readiness probe.
// It implements the Component interface by delegating to the wrapped run function.
func (a *adapter[T]) Run(ctx context.Context, readinessProbe func(cause error)) error {
	return a.run(ctx, a.delegate, readinessProbe)
}
