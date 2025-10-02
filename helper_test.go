package goscade

import (
	"context"
	"testing"
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
