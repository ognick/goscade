package goscade

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockComponentCyclic is used to create a cyclic dependency in tests.
// It implements the Component interface and calls readinessProbe immediately.
type mockComponentCyclic struct {
	dep Component
}

func (m *mockComponentCyclic) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

func runLifecycle(ctx context.Context, lc Lifecycle) error {
	ready := make(chan struct{})
	lc.Run(ctx, func(err error) {
		close(ready)
	})
	select {
	case <-ready:
		return nil
	case <-time.After(1 * time.Second):
		return context.DeadlineExceeded
	}
}

// Test: Circular dependency detection
func TestLifecycle_Run_CircularDependency(t *testing.T) {
	tests := []struct {
		name        string
		opts        []Option
		expectPanic bool
	}{
		{
			name:        "panic on circular dependency by default",
			opts:        nil,
			expectPanic: true,
		},
		{
			name:        "ignore circular dependency when WithCircularDependency is set",
			opts:        []Option{WithCircularDependency()},
			expectPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lc := NewLifecycle(&mockLogger{}, tt.opts...)
			compA := &mockComponentCyclic{}
			compB := &mockComponentCyclic{dep: compA}
			compA.dep = compB // create cycle
			lc.Register(compA)
			lc.Register(compB)

			defer func() {
				rec := recover()
				if tt.expectPanic {
					if rec == nil {
						t.Fatal("expected panic due to circular dependency, but did not panic")
					}
					panicMsg, ok := rec.(string)
					if !ok || !strings.Contains(panicMsg, "circular dependency detected") {
						t.Fatalf("unexpected panic message: %v", rec)
					}
				} else {
					if rec != nil {
						t.Fatalf("did not expect panic, but got: %v", rec)
					}
				}
			}()
			assert.NoError(t, runLifecycle(context.Background(), lc))
		})
	}
}

// Test: Successful pointer registration
func TestLifecycle_Register_Pointer(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &mockComponentCyclic{}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	lc.Register(comp)
}

// Test: Register with implicit dependencies
func TestLifecycle_Register_WithImplicitDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	dep1 := &mockComponentCyclic{}
	dep2 := &mockComponentCyclic{}
	comp := &mockComponentCyclic{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	// Register component with implicit dependencies
	lc.Register(comp, dep1, dep2)

	// Check that all components are registered
	deps := lc.Dependencies()
	if len(deps) != 3 {
		t.Errorf("expected 3 components, got %d", len(deps))
	}

	// Check that the component has the correct dependencies
	compDeps := deps[comp]
	if len(compDeps) != 2 {
		t.Errorf("expected 2 dependencies for main component, got %d", len(compDeps))
	}

	// Check that both dependencies are present
	hasDep1, hasDep2 := false, false
	for _, dep := range compDeps {
		if dep == dep1 {
			hasDep1 = true
		}
		if dep == dep2 {
			hasDep2 = true
		}
	}
	if !hasDep1 {
		t.Error("component should have dependency 1")
	}
	if !hasDep2 {
		t.Error("component should have dependency 2")
	}
}

// Test: Register with no implicit dependencies (backward compatibility)
func TestLifecycle_Register_NoImplicitDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &mockComponentCyclic{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	// Register component without implicit dependencies
	lc.Register(comp)

	// Check that component is registered
	deps := lc.Dependencies()
	if len(deps) != 1 {
		t.Errorf("expected 1 component, got %d", len(deps))
	}

	// Check that component has no dependencies
	compDeps := deps[comp]
	if len(compDeps) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(compDeps))
	}
}

// Test: Register with duplicate implicit dependencies
func TestLifecycle_Register_DuplicateImplicitDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	dep := &mockComponentCyclic{}
	comp := &mockComponentCyclic{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	// Register component with duplicate implicit dependencies
	lc.Register(comp, dep, dep)

	// Check dependencies
	deps := lc.Dependencies()
	if len(deps) != 2 {
		t.Errorf("expected 2 components, got %d", len(deps))
	}

	// Check that the component has only one dependency (duplicates should be deduplicated)
	compDeps := deps[comp]
	if len(compDeps) != 1 {
		t.Errorf("expected 1 dependency for main component (duplicates deduplicated), got %d", len(compDeps))
	}

	if compDeps[0] != dep {
		t.Error("component should have the correct dependency")
	}
}

// Test: Register component twice (should not duplicate)
func TestLifecycle_Register_DuplicateComponent(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &mockComponentCyclic{}
	dep := &mockComponentCyclic{}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	// Register component first time
	lc.Register(comp)

	// Register same component again with implicit dependency
	lc.Register(comp, dep)

	// Check that only one instance of component exists
	deps := lc.Dependencies()
	if len(deps) != 2 {
		t.Errorf("expected 2 components, got %d", len(deps))
	}

	// Check that component has the implicit dependency
	compDeps := deps[comp]
	if len(compDeps) != 1 {
		t.Errorf("expected 1 dependency for main component, got %d", len(compDeps))
	}

	if compDeps[0] != dep {
		t.Error("component should have the correct dependency")
	}
}

// Test: Dependencies with a single component without dependencies
func TestLifecycle_Dependencies_Simple(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &mockComponentCyclic{}
	lc.Register(comp)
	deps := lc.Dependencies()
	if len(deps) != 1 {
		t.Errorf("expected 1 component, got %d", len(deps))
	}
	if len(deps[comp]) != 0 {
		t.Errorf("expected 0 dependencies, got %d", len(deps[comp]))
	}
}

// Test: Dependencies with dependencies
func TestLifecycle_Dependencies_WithDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	compA := &mockComponentCyclic{}
	compB := &mockComponentCyclic{dep: compA}
	lc.Register(compA)
	lc.Register(compB)
	lcImpl := lc.(*lifecycle)
	lcImpl.ptrToComp[reflect.ValueOf(compA).Pointer()] = compA
	lcImpl.ptrToComp[reflect.ValueOf(compB).Pointer()] = compB
	deps := lc.Dependencies()
	if len(deps) != 2 {
		t.Errorf("expected 2 components, got %d", len(deps))
	}
	if len(deps[compB]) != 1 {
		t.Errorf("expected 1 dependency for compB, got %d", len(deps[compB]))
	}
}

// Test: buildCompToParents and buildCompToChildren
func TestLifecycle_BuildCompToParents_And_Children(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	compA := &mockComponentCyclic{}
	compB := &mockComponentCyclic{dep: compA}
	lc.Register(compA)
	lc.Register(compB)
	lcImpl := lc.(*lifecycle)
	parents := lcImpl.buildCompToParents()
	children := lcImpl.buildCompToChildren(parents)
	if len(parents) == 0 || len(children) == 0 {
		t.Error("parents or children map is empty")
	}
}

// Test: Run with no components
func TestLifecycle_NoComponents(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()

	assert.NoError(t, runLifecycle(context.Background(), lc))
}

// Test: Correct status transitions
func TestLifecycle_Status_Transitions(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	lcImpl := lc.(*lifecycle)
	ctx := context.Background()
	if lc.Status() != LifecycleStatusIdle {
		t.Errorf("expected status idle, got %s", lc.Status())
	}
	lcImpl.setStatus(ctx, LifecycleStatusRunning)
	if lc.Status() != LifecycleStatusRunning {
		t.Errorf("expected status running, got %s", lc.Status())
	}
	lcImpl.setStatus(ctx, LifecycleStatusReady)
	if lc.Status() != LifecycleStatusReady {
		t.Errorf("expected status ready, got %s", lc.Status())
	}
	lcImpl.setStatus(ctx, LifecycleStatusStopping)
	if lc.Status() != LifecycleStatusStopping {
		t.Errorf("expected status stopping, got %s", lc.Status())
	}
	lcImpl.setStatus(ctx, LifecycleStatusStopped)
	if lc.Status() != LifecycleStatusStopped {
		t.Errorf("expected status stopped, got %s", lc.Status())
	}
}

// Test: Graceful shutdown
func TestLifecycle_Run_GracefulShutdown(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &mockComponentCyclic{}
	lc.Register(comp)
	lcImpl := lc.(*lifecycle)
	lcImpl.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp
	cancelCtx, cancel := context.WithCancel(context.Background())

	// Start components
	go func() {
		assert.NoError(t, runLifecycle(cancelCtx, lc))
	}()

	// Wait until status becomes Ready
	for lc.Status() != LifecycleStatusReady {
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel context
	cancel()

	// Wait until status becomes Stopped
	for lc.Status() != LifecycleStatusStopped {
		time.Sleep(10 * time.Millisecond)
	}
}

// Test: Component error handling
type errorComponent struct{}

func (e *errorComponent) Run(_ context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	return errors.New("component error")
}

// Test: Component error causes lifecycle to stop
func TestLifecycle_Run_ComponentError(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &errorComponent{}
	lc.Register(comp)
	lcImpl := lc.(*lifecycle)
	lcImpl.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	// Start components
	go func() {
		assert.NoError(t, runLifecycle(context.Background(), lc))
	}()

	// Wait until status becomes Stopped
	for lc.Status() != LifecycleStatusStopped {
		time.Sleep(10 * time.Millisecond)
	}
}

// Test: NewAdapter creates an adapter with correct delegate and run function
func TestNewAdapter_Run(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	var probeCalled bool
	adapter := NewAdapter(&mockComponentCyclic{}, func(ctx context.Context, delegate *mockComponentCyclic, probe func(error)) error {
		probeCalled = true
		probe(nil)
		<-ctx.Done()
		return nil
	})

	assert.NotNil(t, adapter)
	assert.Implements(t, (*Component)(nil), adapter)

	lc := NewLifecycle(&mockLogger{})
	lc.Register(adapter)

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	assert.NoError(t, <-readinessProbe)
	assert.True(t, probeCalled, "Readiness probe should have been called")

	cancel()
	assert.EqualError(t, waitGracefulShutdown(), "context canceled")
}

// Test: adapter delegateName returns correct type string
func TestAdapter_DelegateName(t *testing.T) {
	mockDelegate := &mockComponentCyclic{}
	adapter := NewAdapter(mockDelegate, func(ctx context.Context, delegate *mockComponentCyclic, probe func(error)) error {
		probe(nil)
		<-ctx.Done()
		return nil
	})

	// Cast to adapter to access delegateName method
	adapterImpl := adapter.(interface{ delegateName() string })
	name := adapterImpl.delegateName()

	expectedName := reflect.TypeOf(mockDelegate).String()
	assert.Equal(t, expectedName, name)
}

// Test: adapter Run method handles errors correctly
func TestAdapter_Run_Error(t *testing.T) {
	mockDelegate := &mockComponentCyclic{}
	expectedError := errors.New("test error")

	adapter := NewAdapter(mockDelegate, func(ctx context.Context, delegate *mockComponentCyclic, probe func(error)) error {
		probe(nil)
		return expectedError
	})

	ctx := context.Background()
	err := adapter.Run(ctx, func(err error) {})

	assert.Equal(t, expectedError, err)
}

// customComponent implements delegateNameProvider for testing
type customComponent struct {
	name string
}

func (c *customComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

func (c *customComponent) delegateName() string {
	return c.name
}

// Test: runComponent with delegateNameProvider
func TestRunComponent_DelegateNameProvider(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	comp := &customComponent{name: "CustomComponent"}
	lc.Register(comp)

	// Run lifecycle to test delegateNameProvider functionality
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	assert.NoError(t, <-readinessProbe)

	// Wait for graceful shutdown
	err := waitGracefulShutdown()
	assert.Error(t, err) // Should be context deadline exceeded or canceled
}

// Test: runComponent with parent dependency failure
func TestRunComponent_ParentDependencyFailure(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	// Create parent component that will fail
	parentComp := &errorComponent{}
	childComp := &mockComponentCyclic{dep: parentComp}

	lc.Register(parentComp)
	lc.Register(childComp)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	<-readinessProbe
	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.EqualError(t, shutdownErr, "component error")
}

// cascadeComponent causes cascade shutdown for testing
type cascadeComponent struct{}

func (c *cascadeComponent) Run(_ context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	// Return an error to trigger cascade shutdown
	return errors.New("component error")
}

// Test: runComponent with cascade shutdown
func TestRunComponent_CascadeShutdown(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	comp := &cascadeComponent{}
	lc.Register(comp)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Should get a response from readiness probe
	<-readinessProbe
	// The readiness probe might succeed initially, but the component will fail later
	// So we just check that we get a response (could be nil or error)

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.Error(t, shutdownErr)
}

// probeErrorComponent fails readiness probe for testing
type probeErrorComponent struct{}

func (c *probeErrorComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	// Call readiness probe with error
	readinessProbe(errors.New("probe error"))
	<-ctx.Done()
	return nil
}

// Test: runComponent with probe error
func TestRunComponent_ProbeError(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	comp := &probeErrorComponent{}
	lc.Register(comp)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Should get an error due to probe failure
	err := <-readinessProbe
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "probe error")

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.Error(t, shutdownErr)
}

// unexpectedCloseComponent closes without error for testing
type unexpectedCloseComponent struct{}

func (c *unexpectedCloseComponent) Run(_ context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	// Return nil without waiting for context cancellation
	return nil
}

// Test: runComponent with unexpected close
func TestRunComponent_UnexpectedClose(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	comp := &unexpectedCloseComponent{}
	lc.Register(comp)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Should get a response from readiness probe
	<-readinessProbe
	// The readiness probe might succeed initially, but the component will fail later
	// So we just check that we get a response (could be nil or error)

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.Error(t, shutdownErr)
}

// slowStartComponent takes time to start for timeout testing
type slowStartComponent struct {
	delay time.Duration
}

func (c *slowStartComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	// Wait longer than the timeout before calling readiness probe
	time.Sleep(c.delay)
	readinessProbe(nil)
	<-ctx.Done()
	return ctx.Err()
}

// TestTimeout_StartTimeout tests that components timeout during startup
func TestTimeout_StartTimeout(t *testing.T) {
	lc := NewLifecycle(&mockLogger{}, WithStartTimeout(100*time.Millisecond))

	// Component that takes longer than timeout to start
	comp := &slowStartComponent{delay: 200 * time.Millisecond}
	lc.Register(comp)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Should get timeout error
	err := <-readinessProbe
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.ErrorIs(t, shutdownErr, context.Canceled)
}

// TestTimeout_DefaultTimeouts tests that default timeouts work correctly
func TestTimeout_DefaultTimeouts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	lc := NewLifecycle(&mockLogger{}) // No custom timeouts, should use defaults

	// Component that starts quickly
	comp := &mockComponentCyclic{}
	lc.Register(comp)

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Should succeed with default timeout (1 minute)
	err := <-readinessProbe
	assert.NoError(t, err)

	cancel()

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.ErrorIs(t, shutdownErr, context.Canceled)
}

// TestLifecycle_AsComponent tests that a lifecycle can be registered as a component in another lifecycle
func TestLifecycle_AsComponent(t *testing.T) {
	// Create parent lifecycle
	log := &mockLogger{}
	parentLC := NewLifecycle(log)

	// Create child lifecycle with a component
	childLog := &mockLogger{}
	childLC := NewLifecycle(childLog)
	// Add a component to child lifecycle
	childLC.Register(&mockComponentCyclic{})

	// Wrap child lifecycle as a component
	parentLC.Register(childLC.AsComponent())

	// Add another component to parent lifecycle
	parentComp := &mockComponentCyclic{}
	parentLC.Register(parentComp)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	readinessProbe := make(chan error)
	waitGracefulShutdown := parentLC.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Wait for readiness probe - should succeed
	err := <-readinessProbe
	assert.NoError(t, err)

	// Verify both lifecycles are running
	assert.Equal(t, LifecycleStatusReady, parentLC.Status())
	cancel()

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.ErrorIs(t, shutdownErr, context.Canceled)
}

// TestLifecycle_NestedLifecycleWithDependencies tests nested lifecycles with dependencies
func TestLifecycle_NestedLifecycleWithDependencies(t *testing.T) {
	// Create parent lifecycle
	parentLog := &mockLogger{}
	parentLC := NewLifecycle(parentLog)

	// Create child lifecycle with dependencies
	childLog := &mockLogger{}
	childLC := NewLifecycle(childLog)

	// Add components with dependencies to child lifecycle
	childDB := &mockComponentCyclic{}
	childCache := &mockComponentCyclic{dep: childDB}
	childService := &mockComponentCyclic{dep: childCache}

	childLC.Register(childDB)
	childLC.Register(childCache)
	childLC.Register(childService)

	// Wrap child lifecycle as a component
	parentLC.Register(childLC.AsComponent())

	// Add parent-level component
	parentService := &mockComponentCyclic{}
	parentLC.Register(parentService)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

	readinessProbe := make(chan error)
	waitGracefulShutdown := parentLC.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Wait for readiness probe - should succeed
	err := <-readinessProbe
	assert.NoError(t, err)

	// Verify parent lifecycle is ready
	assert.Equal(t, LifecycleStatusReady, parentLC.Status())
	cancel()

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.ErrorIs(t, shutdownErr, context.Canceled)
}

type startupOrder struct {
	items []string
	mu    sync.Mutex
}

func (c *startupOrder) add(name string) {
	c.mu.Lock()
	c.items = append(c.items, name)
	c.mu.Unlock()
}

// orderTrackingComponent tracks the order of component startup
type orderTrackingComponent struct {
	order *startupOrder
	name  string
}

func (c *orderTrackingComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	c.order.add(c.name)
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

// Test: Component startup order with implicit dependencies
func TestLifecycle_ImplicitDeps_StartupOrder(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	order := &startupOrder{}

	// Create components
	dep1 := &orderTrackingComponent{name: "dep1", order: order}
	dep2 := &orderTrackingComponent{name: "dep2", order: order}
	comp := &orderTrackingComponent{name: "comp", order: order}

	// Register component with implicit dependencies
	lc.Register(comp, dep1, dep2)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	readinessProbe := make(chan error)
	waitGracefulShutdown := lc.Run(ctx, func(err error) {
		readinessProbe <- err
	})

	// Wait for readiness probe
	err := <-readinessProbe
	assert.NoError(t, err)

	// Check that dependencies started before the main component
	assert.Len(t, order.items, 3, "All components should have started")

	// Find positions of components in startup order
	dep1Pos, dep2Pos, compPos := -1, -1, -1
	for i, name := range order.items {
		switch name {
		case "dep1":
			dep1Pos = i
		case "dep2":
			dep2Pos = i
		case "comp":
			compPos = i
		}
	}

	// Dependencies should start before the main component
	assert.True(t, dep1Pos < compPos, "dep1 should start before comp")
	assert.True(t, dep2Pos < compPos, "dep2 should start before comp")

	cancel()

	// Wait for graceful shutdown
	shutdownErr := waitGracefulShutdown()
	assert.ErrorIs(t, shutdownErr, context.Canceled)
}
