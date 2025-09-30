package goscade

import "context"

// Register is a convenience function that registers a component with the lifecycle
// manager and returns the same component. This allows for fluent-style registration
// and makes it easier to chain component creation and registration.
//
// Example:
//
//	server := Register(lc, NewServer(handler))
//	db := Register(lc, NewDatabase(config))
//
// This is equivalent to:
//
//	server := NewServer(handler)
//	lc.Register(server)
//	db := NewDatabase(config)
//	lc.Register(db)
func Register[T Component](lc Lifecycle, component T) T {
	lc.Register(component)
	return component
}

type lifecycleAsComponent struct {
	lc Lifecycle
}

func LifecycleAsComponent(lc Lifecycle) Component {
	return &lifecycleAsComponent{lc: lc}
}

func (l *lifecycleAsComponent) Run(ctx context.Context, readinessProbe func(cause error)) error {
	stop := l.lc.Run(ctx, readinessProbe)
	<-ctx.Done()
	return stop()
}
