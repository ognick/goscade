package goscade

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"reflect"
	"sync"
	"sync/atomic"
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

	// ShutdownTimeoutError is returned when components do not stop before the
	// configured shutdown timeout. It wraps context.DeadlineExceeded.
	ShutdownTimeoutError = fmt.Errorf("shutdown timeout: %w", context.DeadlineExceeded)
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

	// BuildGraph constructs a visual graph representation based on component dependencies.
	// Returns a Graph structure containing all nodes (components) and edges (dependencies).
	BuildGraph() Graph

	// Register adds a component to the lifecycle manager.
	// The component must be a pointer or interface type.
	// Optional implicitDeps allows explicit dependency declaration when automatic
	// detection is not sufficient (e.g., interface dependencies, function parameters).
	Register(component Component, implicitDeps ...Component)

	// Run starts all registered components and blocks until shutdown.
	// The method handles dependency resolution, concurrent startup, and graceful shutdown.
	// The readinessProbe callback is called when all components are ready or if there's an error during startup.
	// By default, the lifecycle will not respond to system signals unless WithShutdownHook() option is used.
	// The returned error joins the primary shutdown cause with independent component errors.
	// Component errors are wrapped with the component name and remain discoverable with errors.Is.
	Run(ctx context.Context, readinessProbe func(err error)) error

	// Status returns the current status of the lifecycle manager.
	Status() LifecycleStatus
}

// lifecycle is the internal implementation of the Lifecycle interface.
type lifecycle struct {
	mu                 sync.RWMutex
	status             LifecycleStatus
	compToImplicitDeps map[Component]map[Component]struct{}
	components         map[Component]struct{}
	ptrToComp          map[uintptr]Component
	log                logger

	ignoreCircularDependency bool
	shutdownHook             bool
	startTimeout             time.Duration
	shutdownTimeout          time.Duration
	graphOutputFile          string
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

// WithShutdownTimeout sets the maximum time to wait for components to stop.
// Default is 1 minute. A timed-out component goroutine cannot be forcibly
// terminated and may continue until it returns or the process exits.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(lc *lifecycle) {
		lc.shutdownTimeout = timeout
	}
}

// WithGraphOutput enables writing the dependency graph to a file in DOT format.
// The file will be written when the lifecycle starts running.
// Use Graphviz (e.g., dot -Tpng graph.dot -o graph.png) to visualize the output.
func WithGraphOutput(filename string) Option {
	return func(lc *lifecycle) {
		lc.graphOutputFile = filename
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
		components:         make(map[Component]struct{}),
		ptrToComp:          make(map[uintptr]Component),
		startTimeout:       time.Minute, // Default 1 minute
		shutdownTimeout:    time.Minute, // Default 1 minute
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
func (lc *lifecycle) setStatus(newStatus LifecycleStatus) bool {
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

	lc.status = newStatus
	return true
}

// Status returns the current status of the lifecycle manager.
func (lc *lifecycle) Status() LifecycleStatus {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.status
}

// componentName returns the display name for a component.
func (lc *lifecycle) componentName(comp Component) string {
	if a, ok := comp.(delegateNameProvider); ok {
		return a.delegateName()
	}
	return reflect.TypeOf(comp).String()
}

// componentState holds the runtime state for a component including
// its contexts, cancellation functions, and synchronization primitives.
type componentState struct {
	componentName  string
	started        atomic.Bool
	probeCtx       context.Context
	cancelProbe    context.CancelCauseFunc
	runCtx         context.Context
	cancelRun      context.CancelCauseFunc
	teardownCtx    context.Context
	cancelTeardown context.CancelCauseFunc
}

type componentErrors struct {
	mu   sync.Mutex
	errs []error
}

func (e *componentErrors) add(err error) {
	if err == nil {
		return
	}
	e.mu.Lock()
	e.errs = append(e.errs, err)
	e.mu.Unlock()
}

func (e *componentErrors) snapshot() []error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]error(nil), e.errs...)
}

func joinLifecycleErrors(primary error, componentErrs []error) error {
	errs := make([]error, 0, len(componentErrs)+1)
	if primary != nil {
		errs = append(errs, primary)
	}

	for _, err := range componentErrs {
		if err == nil || errors.Is(err, CascadeCloseComponentError) {
			continue
		}
		if primary != nil && (errors.Is(err, primary) || errors.Is(primary, err)) {
			continue
		}
		errs = append(errs, err)
	}

	return errors.Join(errs...)
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
	componentErrs *componentErrors,
) {
	state := compStates[comp]
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
			if !state.started.Load() {
				return err
			}
			probeErr := fmt.Errorf("component %s readiness: %w", state.componentName, err)
			lc.log.Errorf("Component %s [PROB ERROR]: %v", state.componentName, err)
			lifecycleCtxCancel(probeErr)
			return probeErr
		}

		lc.log.Infof("Component %s [READY]", state.componentName)
		return nil
	})

	runner.Go(func() (runErr error) {
		defer state.cancelTeardown(runErr)
		<-startLatch

		for parentComp := range compToParents[comp] {
			if err := waitCtxErr(compStates[parentComp].probeCtx); err != nil {
				state.cancelProbe(err)
				state.cancelRun(err)
				return err
			}
		}

		state.started.Store(true)
		err := comp.Run(state.runCtx, func(err error) {
			state.cancelProbe(err)
			if err != nil {
				lifecycleCtxCancel(fmt.Errorf("component %s readiness: %w", state.componentName, err))
			}
		})
		if err == nil {
			if lifecycleCtx.Err() == nil {
				unexpectedErr := fmt.Errorf("component %s: %w", state.componentName, UnexpectedCloseComponentError)
				componentErrs.add(unexpectedErr)
				lifecycleCtxCancel(unexpectedErr)
			}
		} else {
			propagated := state.runCtx.Err() != nil && errors.Is(err, context.Cause(state.runCtx))
			duplicatePrimary := errors.Is(context.Cause(lifecycleCtx), err)
			if !propagated && !duplicatePrimary && !errors.Is(err, CascadeCloseComponentError) {
				componentErr := fmt.Errorf("component %s: %w", state.componentName, err)
				componentErrs.add(componentErr)
				lifecycleCtxCancel(componentErr)
			}
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
//
// Run returns the cause that initiated shutdown joined with any independent
// component errors. A component that returns nil before shutdown produces
// UnexpectedCloseComponentError. If shutdown exceeds the configured timeout,
// the result includes ShutdownTimeoutError. A timed-out component goroutine
// cannot be forcibly terminated and may continue until it returns or the
// process exits.
func (lc *lifecycle) Run(ctx context.Context, readinessProbe func(err error)) error {
	// Graceful shutdown on context cancellation or signal
	if lc.shutdownHook {
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
	}
	lifecycleCtx, lifecycleCtxCancel := context.WithCancelCause(ctx)
	defer lifecycleCtxCancel(context.Canceled)
	compToParents := lc.buildCompToParents()
	compToChildren := lc.buildCompToChildren(compToParents)

	if err := lc.writeGraphToFile(); err != nil {
		lc.log.Errorf("Failed to write graph: %v", err)
	}
	if len(lc.components) == 0 {
		lc.setStatus(LifecycleStatusRunning)
		lc.setStatus(LifecycleStatusReady)
		if readinessProbe != nil {
			readinessProbe(nil)
		}
		lc.setStatus(LifecycleStatusStopped)
		return nil
	}
	runner := &errgroup.Group{}
	prober := &errgroup.Group{}
	componentErrs := &componentErrors{}
	startLatch := make(chan struct{})
	compStates := make(map[Component]*componentState)
	for comp := range lc.components {
		state := &componentState{}
		compStates[comp] = state
		state.probeCtx, state.cancelProbe = context.WithCancelCause(lifecycleCtx)

		state.runCtx, state.cancelRun = context.WithCancelCause(context.Background())

		state.teardownCtx, state.cancelTeardown = context.WithCancelCause(context.Background())
		state.componentName = lc.componentName(comp)
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
			componentErrs,
		)
	}

	// Wait until all components are stopped
	go func() {
		if err := waitCtxErr(lifecycleCtx); err != nil {
			lc.log.Errorf("All components are stopping: %v", err)
		} else {
			lc.log.Infof("All components are stopping")
		}
		lc.setStatus(LifecycleStatusStopping)
	}()

	// Wait until all probes are done (either ready or failed)
	go func() {
		probeErr := prober.Wait()
		if probeErr == nil || errors.Is(probeErr, context.Canceled) {
			lc.setStatus(LifecycleStatusReady)
			probeErr = nil
		} else if cause := context.Cause(lifecycleCtx); cause != nil {
			probeErr = cause
		}

		if readinessProbe != nil {
			readinessProbe(probeErr)
		}
	}()

	lc.setStatus(LifecycleStatusRunning)
	close(startLatch)

	// Wait until all components are done. The buffered channel lets this waiter
	// finish even if Run has already returned because shutdown timed out.
	runnerDone := make(chan error, 1)
	go func() {
		runErr := runner.Wait()
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			lc.log.Errorf("All components are stopped: %v", runErr)
		} else {
			lc.log.Infof("All components are stopped")
		}

		lc.setStatus(LifecycleStatusStopped)
		runnerDone <- runErr
	}()

	var timeoutErr error
	select {
	case <-runnerDone:
	case <-lifecycleCtx.Done():
		timer := time.NewTimer(lc.shutdownTimeout)
		select {
		case <-runnerDone:
		case <-timer.C:
			timeoutErr = ShutdownTimeoutError
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}

	primaryErr := context.Cause(lifecycleCtx)
	errs := componentErrs.snapshot()
	if timeoutErr != nil {
		errs = append(errs, timeoutErr)
	}
	return joinLifecycleErrors(primaryErr, errs)
}
