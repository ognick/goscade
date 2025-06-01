package goscade

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

var (
	UnexpectedCloseComponentError = errors.New("unexpected close component")
	// GracefulCloseComponentError = errors.New("graceful close component")
	CascadeCloseComponentError = errors.New("cascade close component")
)

type logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type Component interface {
	Run(ctx context.Context, readinessProbe func(cause error)) error
}

type LifecycleStatus string

const (
	LifecycleStatusIdle     LifecycleStatus = "idle"
	LifecycleStatusRunning  LifecycleStatus = "running"
	LifecycleStatusReady    LifecycleStatus = "ready"
	LifecycleStatusStopping LifecycleStatus = "stopping"
	LifecycleStatusStopped  LifecycleStatus = "stopped"
)

type Lifecycle interface {
	Dependencies() map[Component][]Component
	Register(component Component)
	RunAllComponents(runner runner, gracefulCtx context.Context)
	Status() LifecycleStatus
}

type lifecycle struct {
	mu             sync.RWMutex
	status         LifecycleStatus
	statusListener chan LifecycleStatus
	components     map[Component]struct{}
	ptrToComp      map[uintptr]Component
	log            logger
}

func NewLifecycle(log logger) Lifecycle {
	return &lifecycle{
		log:            log,
		status:         LifecycleStatusIdle,
		statusListener: make(chan LifecycleStatus),
		components:     make(map[Component]struct{}),
		ptrToComp:      make(map[uintptr]Component),
	}
}

func (lc *lifecycle) Register(comp Component) {
	val := reflect.ValueOf(comp)
	if val.Kind() != reflect.Pointer {
		panic(fmt.Sprintf("component must be a pointer, got %s", val.Kind()))
	}

	lc.components[comp] = struct{}{}
	lc.ptrToComp[val.Pointer()] = comp
}

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

func (lc *lifecycle) Status() LifecycleStatus {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.status
}

type runner interface {
	Go(f func() error)
}

type componentState struct {
	probeCtx    context.Context
	cancelProbe context.CancelCauseFunc
	runCtx      context.Context
	cancelRun   context.CancelCauseFunc

	allChildrenClosed sync.WaitGroup
}

//nolint:gocyclo
func (lc *lifecycle) RunAllComponents(
	runner runner,
	gracefulCtx context.Context,
) {
	lc.setStatus(context.Background(), LifecycleStatusRunning)

	lifecycleCtx, lifecycleCtxCancel := context.WithCancelCause(gracefulCtx)
	rootStates := make([]*componentState, 0)
	leafStates := make([]*componentState, 0)

	compToParents := lc.buildCompToParents()
	compToChildren := lc.buildCompToChildren(compToParents)

	compStates := make(map[Component]*componentState)
	for comp := range lc.components {
		state := &componentState{}
		state.probeCtx, state.cancelProbe = context.WithCancelCause(lifecycleCtx)
		state.runCtx, state.cancelRun = context.WithCancelCause(context.Background())

		compStates[comp] = state
		if parents, ok := compToParents[comp]; !ok || len(parents) == 0 {
			rootStates = append(rootStates, state)
		}

		children, ok := compToChildren[comp]
		// If the component has children, we can close it only when all children are closed
		if ok && len(children) > 0 {
			state.allChildrenClosed.Add(len(children))
			continue
		}
		// Leaf component, no children
		leafStates = append(leafStates, state)
		// Fake children to ensure that the component is closed only when graceful context is done
		state.allChildrenClosed.Add(1)
	}

	// stop leaf components when lifecycle context is done
	go func() {
		<-lifecycleCtx.Done()
		lc.setStatus(gracefulCtx, LifecycleStatusStopping)
		for _, s := range leafStates {
			s.allChildrenClosed.Done()
		}
	}()

	// wait for leaf components to finish their readiness probes
	go func() {
		var probErr error
		for _, state := range leafStates {
			<-state.probeCtx.Done()
			if err := context.Cause(state.probeCtx); err != nil && !errors.Is(err, context.Canceled) {
				probErr = err
			}
		}

		if probErr == nil {
			lc.setStatus(gracefulCtx, LifecycleStatusReady)
		}
	}()

	go func() {
		for _, s := range rootStates {
			<-s.runCtx.Done()
		}

		lc.setStatus(gracefulCtx, LifecycleStatusStopped)
		lc.log.Infof("All components are stopped")
	}()

	running := make(map[Component]struct{})
	for len(running) < len(lc.components) {
		readyForRun := make(map[Component]struct{}, len(lc.components)-len(running))
		for comp := range lc.components {
			if _, ok := running[comp]; ok {
				continue
			}

			parents, ok := compToParents[comp]
			if !ok || len(parents) == 0 {
				readyForRun[comp] = struct{}{}
				continue
			}

			areAllParentsRunning := true
			for parent := range parents {
				if _, ok := running[parent]; !ok {
					areAllParentsRunning = false
					break
				}
			}

			if areAllParentsRunning {
				readyForRun[comp] = struct{}{}
			}
		}

		if len(readyForRun) == 0 {
			panic("circular dependency detected")
		}

		for comp := range readyForRun {
			state := compStates[comp]
			go func() {
				state.allChildrenClosed.Wait()
				state.cancelRun(context.Cause(lifecycleCtx))
			}()

			componentName := reflect.TypeOf(comp).String()
			running[comp] = struct{}{}

			runner.Go(func() (runErr error) {
				if parents, ok := compToParents[comp]; ok {
					// Send close signal to all parents when this component is closed
					defer func() {
						state.cancelRun(runErr)
						state.cancelProbe(runErr)
						for parent := range parents {
							compStates[parent].allChildrenClosed.Done()
						}
					}()

					//Wait parent probes
					for parent := range parents {
						parentState := compStates[parent]
						<-parentState.probeCtx.Done()
						if err := context.Cause(parentState.probeCtx); err != nil && !errors.Is(err, context.Canceled) {
							return err
						}
					}
				}

				err := comp.Run(state.runCtx, state.cancelProbe)
				if context.Cause(lifecycleCtx) == nil {
					if err == nil {
						err = UnexpectedCloseComponentError
					}
					lifecycleCtxCancel(CascadeCloseComponentError)
				}
				if err == nil {
					err = context.Cause(lifecycleCtx)
				}

				switch {
				case errors.Is(err, CascadeCloseComponentError):
					lc.log.Infof("Component %s [CASCADE]", componentName)
				case errors.Is(err, context.Canceled):
					lc.log.Infof("Component %s [CLOSE]", componentName)
				case errors.Is(err, nil):
					lc.log.Infof("Component %s [CLOSE]", componentName)
				default:
					lc.log.Errorf("Component %s [ERROR] %v", componentName, err)
				}

				return err
			})

			go func() {
				select {
				case <-state.probeCtx.Done():
					if err := context.Cause(state.probeCtx); err != nil && !errors.Is(err, context.Canceled) {
						lc.log.Errorf("Component %s [PROB ERROR]: %v", componentName, err)
						lifecycleCtxCancel(CascadeCloseComponentError)
						return
					}
					lc.log.Infof("Component %s [READY]", componentName)
				}
			}()
		}
	}
}
