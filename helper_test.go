package goscade

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRegister tests the Register helper function.
func TestRegister(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := &mockComponentCyclic{}

	// Test that Register returns the same component
	result := Register(lc, comp)
	if result != comp {
		t.Error("Register should return the same component")
	}

	// Test that component is actually registered
	deps := lc.Dependencies()
	if len(deps) != 1 {
		t.Errorf("expected 1 component, got %d", len(deps))
	}
	if _, exists := deps[comp]; !exists {
		t.Error("component should be registered in lifecycle")
	}
}

// TestRegister_Nil tests Register with nil component.
func TestRegister_Nil(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when registering nil component")
		}
	}()
	Register[Component](lc, nil)
}

// nonPointerComponent is a component with value receiver for Run method.
type nonPointerComponent struct{}

func (c nonPointerComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	readinessProbe(nil)
	<-ctx.Done()
	return nil
}

// TestRegister_NonPointer tests Register with non-pointer component.
func TestRegister_NonPointer(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})
	comp := nonPointerComponent{} // Create value, not pointer
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when registering non-pointer component")
		}
	}()
	Register[Component](lc, comp)
}

// TestRegister_WithImplicitDeps tests Register with implicit dependencies.
func TestRegister_WithImplicitDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	// Create components
	dep1 := &mockComponentCyclic{}
	dep2 := &mockComponentCyclic{}
	comp := &mockComponentCyclic{}

	// Register component with implicit dependencies
	result := Register(lc, comp, dep1, dep2)
	if result != comp {
		t.Error("Register should return the same component")
	}

	// Check that all components are registered
	deps := lc.Dependencies()
	if len(deps) != 3 {
		t.Errorf("expected 3 components, got %d", len(deps))
	}

	// Check that implicit dependencies are correctly set
	if _, exists := deps[comp]; !exists {
		t.Error("main component should be registered")
	}
	if _, exists := deps[dep1]; !exists {
		t.Error("dependency 1 should be registered")
	}
	if _, exists := deps[dep2]; !exists {
		t.Error("dependency 2 should be registered")
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

// TestRegister_WithNestedImplicitDeps tests Register with nested implicit dependencies.
func TestRegister_WithNestedImplicitDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	// Create components
	baseDep := &mockComponentCyclic{}
	midDep := &mockComponentCyclic{}
	topComp := &mockComponentCyclic{}

	// Register base dependency
	Register(lc, baseDep)

	// Register mid component with base dependency
	Register(lc, midDep, baseDep)

	// Register top component with mid dependency
	Register(lc, topComp, midDep)

	// Check dependencies
	deps := lc.Dependencies()
	if len(deps) != 3 {
		t.Errorf("expected 3 components, got %d", len(deps))
	}

	// Check that mid component has base dependency
	midDeps := deps[midDep]
	if len(midDeps) != 1 || midDeps[0] != baseDep {
		t.Error("mid component should have base dependency")
	}

	// Check that top component has mid dependency
	topDeps := deps[topComp]
	if len(topDeps) != 1 || topDeps[0] != midDep {
		t.Error("top component should have mid dependency")
	}
}

// TestRegister_WithDuplicateImplicitDeps tests Register with duplicate implicit dependencies.
func TestRegister_WithDuplicateImplicitDeps(t *testing.T) {
	lc := NewLifecycle(&mockLogger{})

	// Create components
	dep := &mockComponentCyclic{}
	comp := &mockComponentCyclic{}

	// Register component with duplicate implicit dependencies
	Register(lc, comp, dep, dep)

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

// TestRun tests the Run helper function with successful component execution.
func TestRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	readyCalled := false
	comp := &mockComponentCyclic{}

	err := Run(ctx, comp, func() {
		readyCalled = true
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	assert.True(t, readyCalled, "ready callback should be called")
}

// TestRun_ComponentError tests the Run helper function with component error.
func TestRun_ComponentError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	comp := &errorComponent{}
	err := Run(ctx, comp, func() {
		t.Error("ready callback should not be called on component error")
	})

	if err == nil {
		t.Error("expected error, got nil")
	} else {
		if !strings.Contains(err.Error(), "component error") {
			t.Errorf("expected error to contain 'component error', got %v", err)
		}
	}
}

// TestRun_ProbeError tests the Run helper function with readiness probe error.
func TestRun_ProbeError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	comp := &probeErrorComponent{}

	err := Run(ctx, comp, func() {
		t.Error("ready callback should not be called on probe error")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "probe error") {
		t.Errorf("expected error to contain 'probe error', got %v", err)
	}
}

// TestRun_Timeout tests the Run helper function with timeout.
func TestRun_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Component that takes longer than timeout
	comp := &slowStartComponent{delay: 100 * time.Millisecond}

	err := Run(ctx, comp, func() {
		t.Error("ready callback should not be called on timeout")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("expected error to contain 'context deadline exceeded', got %v", err)
	}
}
