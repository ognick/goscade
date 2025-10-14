package goscade

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"reflect"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	// UnexpectedCloseComponentError is returned when a component closes unexpectedly
	// without being explicitly stopped by the lifecycle manager.
	UnexpectedCloseComponentError = errors.New("unexpected close component")

	// CascadeCloseComponentError is returned when a component is closed as part
	// of a cascade shutdown initiated by another component's failure.
	CascadeCloseComponentError = errors.New("cascade close component")
)

// logger defines the interface for logging within the lifecycle system.
type logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// Component represents a component that can be managed by the lifecycle system.
// Each component must implement the Run method which will be called by the
// lifecycle manager to start the component.
type Component interface {
	// Run starts the component with the provided context and readiness probe.
	// The readinessProbe function should be called when the component is ready
	// to serve requests. If called with an error, the component will be marked
	// as failed and the lifecycle will initiate a shutdown.
	Run(ctx context.Context, readinessProbe func(cause error)) error
}

// LifecycleStatus represents the current state of the lifecycle manager.
type LifecycleStatus string

const (
	// LifecycleStatusIdle indicates the lifecycle is not running any components.
	LifecycleStatusIdle LifecycleStatus = "idle"

	// LifecycleStatusRunning indicates components are starting up.
	LifecycleStatusRunning LifecycleStatus = "running"

	// LifecycleStatusReady indicates all components are running and ready.
	LifecycleStatusReady LifecycleStatus = "ready"

	// LifecycleStatusStopping indicates components are shutting down.
	LifecycleStatusStopping LifecycleStatus = "stopping"

	// LifecycleStatusStopped indicates all components have been stopped.
	LifecycleStatusStopped LifecycleStatus = "stopped"
)

// Lifecycle manages the lifecycle of components, including their startup,
// dependency resolution, and graceful shutdown.
type Lifecycle interface {
	// Dependencies returns a map showing the dependency graph of all registered components.
	Dependencies() map[Component][]Component

	// Register adds a component to the lifecycle manager.
	// The component must be a pointer or interface type.
	// Optional implicitDeps allows explicit dependency declaration when automatic
	// detection is not sufficient (e.g., interface dependencies, function parameters).
	Register(component Component, implicitDeps ...Component)

	// Run starts all registered components and blocks until shutdown.
	// The method handles dependency resolution, concurrent startup, and graceful shutdown.
	// The readinessProbe callback is called when all components are ready or if there's an error during startup.
	// By default, the lifecycle will not respond to system signals unless WithShutdownHook() option is used.
	Run(ctx context.Context, readinessProbe func(err error)) error

	// Status returns the current status of the lifecycle manager.
	Status() LifecycleStatus
}

// lifecycle is the internal implementation of the Lifecycle interface.
type lifecycle struct {
	mu                 sync.RWMutex
	status             LifecycleStatus
	statusListener     chan LifecycleStatus
	compToImplicitDeps map[Component]map[Component]struct{}
	components         map[Component]struct{}
	ptrToComp          map[uintptr]Component
	log                logger

	ignoreCircularDependency bool
	shutdownHook             bool
	startTimeout             time.Duration
}

// Option is a function type for configuring lifecycle behavior.
type Option func(*lifecycle)

// WithCircularDependency enables support for circular dependencies.
// WARNING: This option should be used with caution as it can lead to
// unpredictable behavior and potential deadlocks. Only use this if you
// have a specific need and understand the implications.
func WithCircularDependency() Option {
	return func(lc *lifecycle) {
		lc.ignoreCircularDependency = true
	}
}

// WithShutdownHook enables graceful shutdown on system signals (SIGINT, SIGTERM).
// By default, lifecycles do not respond to system signals and only shut down
// when the context is cancelled. This option enables signal handling for
// graceful shutdown on system termination signals.
func WithShutdownHook() Option {
	return func(lc *lifecycle) {
		lc.shutdownHook = true
	}
}

// WithStartTimeout sets the timeout for component startup and readiness probe.
// Default is 1 minute.
func WithStartTimeout(timeout time.Duration) Option {
	return func(lc *lifecycle) {
		lc.startTimeout = timeout
	}
}

// NewLifecycle creates a new lifecycle manager with the provided logger and options.
// The lifecycle manager will handle component registration, dependency resolution,
// and graceful shutdown of all registered components.
func NewLifecycle(log logger, opts ...Option) Lifecycle {
	lc := &lifecycle{
		log:                log,
		status:             LifecycleStatusIdle,
		compToImplicitDeps: make(map[Component]map[Component]struct{}),
		statusListener:     make(chan LifecycleStatus),
		components:         make(map[Component]struct{}),
		ptrToComp:          make(map[uintptr]Component),
		startTimeout:       time.Minute, // Default 1 minute
	}

	for _, opt := range opts {
		opt(lc)
	}

	return lc
}

// Register adds a component to the lifecycle manager.
// The component must be a pointer type for proper dependency detection.
// This method will panic if a non-pointer component is registered.
// Optional implicitDeps allows explicit dependency declaration when automatic
// detection is not sufficient (e.g., interface dependencies, function parameters).
func (lc *lifecycle) Register(comp Component, implicitDeps ...Component) {
	if _, ok := lc.components[comp]; !ok {
		val := reflect.ValueOf(comp)
		if val.Kind() != reflect.Pointer {
			panic(fmt.Sprintf("component must be a pointer, got %s", val.Kind()))
		}

		lc.components[comp] = struct{}{}
		lc.ptrToComp[val.Pointer()] = comp
		lc.compToImplicitDeps[comp] = make(map[Component]struct{})
	}

	for _, dep := range implicitDeps {
		lc.Register(dep)
		lc.compToImplicitDeps[comp][dep] = struct{}{}
	}
}

// setStatus updates the lifecycle status with proper state transition validation.
// It returns true if the status change was successful, false if the transition
// is not allowed from the current state.
func (lc *lifecycle) setStatus(ctx context.Context, newStatus LifecycleStatus) bool {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	switch newStatus {
	case LifecycleStatusStopping:
		if lc.status != LifecycleStatusRunning && lc.status != LifecycleStatusReady {
			return false
		}
	case LifecycleStatusReady:
		if lc.status != LifecycleStatusRunning {
			return false
		}
	}

	go func() {
		select {
		case <-ctx.Done():
		case lc.statusListener <- newStatus:
		}
	}()

	lc.status = newStatus
	return true
}

// Status returns the current status of the lifecycle manager.
func (lc *lifecycle) Status() LifecycleStatus {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.status
}

// componentState holds the runtime state for a component including
// its contexts, cancellation functions, and synchronization primitives.
type componentState struct {
	componentName  string
	probeCtx       context.Context
	cancelProbe    context.CancelCauseFunc
	runCtx         context.Context
	cancelRun      context.CancelCauseFunc
	teardownCtx    context.Context
	cancelTeardown context.CancelCauseFunc
}

// waitCtxErr waits for a context to be done and returns the cause of cancellation.
// If the context was canceled (not timed out), it returns nil.
func waitCtxErr(ctx context.Context) error {
	<-ctx.Done()
	err := context.Cause(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// runComponent starts a component and manages its lifecycle including
// dependency waiting, readiness probing, and graceful shutdown.
func (lc *lifecycle) runComponent(
	lifecycleCtx context.Context,
	lifecycleCtxCancel context.CancelCauseFunc,
	comp Component,
	runner *errgroup.Group,
	prober *errgroup.Group,
	compStates map[Component]*componentState,
	compToParents map[Component]map[Component]struct{},
	compToChildren map[Component]map[Component]struct{},
	startLatch chan struct{},
) {
	state := compStates[comp]
	//  Wait until all parents have started successfully, or any of them has failed
	waitAllParentProbes := make(chan error)
	go func() {
		for parentComp := range compToParents[comp] {
			if err := waitCtxErr(compStates[parentComp].probeCtx); err != nil {
				waitAllParentProbes <- err
				break
			}
		}
		waitAllParentProbes <- nil
	}()

	//  Wait until all children have finished successfully, or any of them has failed
	go func() {
		for childComp := range compToChildren[comp] {
			if err := waitCtxErr(compStates[childComp].teardownCtx); err != nil {
				state.cancelRun(err)
				break
			}
		}

		state.cancelRun(waitCtxErr(lifecycleCtx))
	}()

	// Wait until the component's readiness probe signals ready or failed
	prober.Go(func() error {
		probeCtx, cancel := context.WithTimeout(state.probeCtx, lc.startTimeout)
		defer cancel()

		if err := waitCtxErr(probeCtx); err != nil {
			lc.log.Errorf("Component %s [PROB ERROR]: %v", state.componentName, err)
			lifecycleCtxCancel(CascadeCloseComponentError)
			return err
		}

		lc.log.Infof("Component %s [READY]", state.componentName)
		return nil
	})

	runner.Go(func() (runErr error) {
		defer state.cancelTeardown(runErr)
		<-startLatch

		err := <-waitAllParentProbes
		if err != nil && !errors.Is(err, context.Canceled) {
			state.cancelProbe(err)
			state.cancelRun(err)
			return err
		}

		err = comp.Run(state.runCtx, state.cancelProbe)
		if err == nil {
			lifecycleCtxCancel(UnexpectedCloseComponentError)
		} else {
			lifecycleCtxCancel(err)
		}

		switch {
		case errors.Is(err, CascadeCloseComponentError):
			lc.log.Infof("Component %s [CASCADE]", state.componentName)
		case errors.Is(err, context.Canceled):
			lc.log.Infof("Component %s [CLOSE]", state.componentName)
		case errors.Is(err, nil):
			lc.log.Infof("Component %s [CLOSE]", state.componentName)
		default:
			lc.log.Errorf("Component %s [ERROR] %v", state.componentName, err)
		}

		return err
	})
}

// Run starts all registered components and blocks until shutdown.
// The method handles:
// - Dependency resolution and topological sorting
// - Concurrent component startup
// - Readiness probing and status management
// - Error propagation and cascade shutdown
// - Graceful shutdown on context cancellation
//
// The readinessProbe callback is called when all components are ready
// or if there's an error during startup.
// By default, the lifecycle will not respond to system signals unless
// WithShutdownHook() option is used during lifecycle creation.
func (lc *lifecycle) Run(ctx context.Context, readinessProbe func(err error)) error {
	// Graceful shutdown on context cancellation or signal
	if lc.shutdownHook {
		ctx, _ = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	}
	lifecycleCtx, lifecycleCtxCancel := context.WithCancelCause(ctx)
	compToParents := lc.buildCompToParents()
	compToChildren := lc.buildCompToChildren(compToParents)
	runner := &errgroup.Group{}
	prober := &errgroup.Group{}
	startLatch := make(chan struct{})
	compStates := make(map[Component]*componentState)
	for comp := range lc.components {
		state := &componentState{}
		compStates[comp] = state
		state.probeCtx, state.cancelProbe = context.WithCancelCause(lifecycleCtx)

		state.runCtx, state.cancelRun = context.WithCancelCause(context.Background())

		state.teardownCtx, state.cancelTeardown = context.WithCancelCause(context.Background())
		state.componentName = reflect.TypeOf(comp).String()
		if a, ok := comp.(delegateNameProvider); ok {
			state.componentName = a.delegateName()
		}
	}

	for comp := range lc.components {
		lc.runComponent(
			lifecycleCtx,
			lifecycleCtxCancel,
			comp,
			runner,
			prober,
			compStates,
			compToParents,
			compToChildren,
			startLatch,
		)
	}

	// Wait until all components are stopped
	go func() {
		if err := waitCtxErr(lifecycleCtx); err != nil {
			lc.log.Errorf("All components are stopping: %v", err)
		} else {
			lc.log.Infof("All components are stopping")
		}
		lc.setStatus(ctx, LifecycleStatusStopping)
	}()

	// Wait until all probes are done (either ready or failed)
	go func() {
		probeErr := prober.Wait()
		if probeErr == nil || errors.Is(probeErr, context.Canceled) {
			lc.setStatus(ctx, LifecycleStatusReady)
			probeErr = nil
		}

		if readinessProbe != nil {
			readinessProbe(probeErr)
		}
	}()

	// Wait until all components are done
	teardownCtx, cancelTeardown := context.WithCancelCause(context.Background())
	go func() {
		err := runner.Wait()
		if err != nil && !errors.Is(err, context.Canceled) {
			lc.log.Errorf("All components are stopped: %v", err)
		} else {
			lc.log.Infof("All components are stopped")
		}

		lc.setStatus(ctx, LifecycleStatusStopped)
		cancelTeardown(err)
	}()

	lc.setStatus(ctx, LifecycleStatusRunning)
	close(startLatch)

	<-teardownCtx.Done()
	lifecycleCtxCancel(context.Canceled)
	return context.Cause(teardownCtx)
}
