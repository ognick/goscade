package goscade

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockComponent implements Component interface for testing
type mockComponent struct {
	name string
}

func (m *mockComponent) Run(ctx context.Context, readinessProbe func(error)) error {
	return nil
}

// TestStruct implements Component interface for testing
type TestStruct struct {
	Dep1 *mockComponent
	Dep2 *mockComponent
}

func (t *TestStruct) Run(ctx context.Context, readinessProbe func(cause error)) error {
	return nil
}

// ComplexStruct implements Component interface for testing
type ComplexStruct struct {
	Dep1 *mockComponent
	Dep2 *mockComponent
	Dep3 *mockComponent
}

func (c *ComplexStruct) Run(ctx context.Context, readinessProbe func(cause error)) error {
	return nil
}

// RecStruct is used to test recursive structures
type RecStruct struct {
	Name string
	Comp *mockComponent
	Dep  *RecStruct
}

func (r *RecStruct) Run(ctx context.Context, readinessProbe func(cause error)) error {
	return nil
}

// mockLogger implements logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Infof(format string, args ...interface{})  {}
func (m *mockLogger) Errorf(format string, args ...interface{}) {}

// setupTestLifecycle creates a new lifecycle for testing
func setupTestLifecycle() *lifecycle {
	return &lifecycle{
		components: make(map[Component]struct{}),
		ptrToComp:  make(map[uintptr]Component),
		log:        &mockLogger{},
	}
}

// TestFindParentComponents_Empty tests findParentComponents with empty values
func TestFindParentComponents_Empty(t *testing.T) {
	lc := setupTestLifecycle()
	parents := lc.findParentComponents(nil)
	if len(parents) != 0 {
		t.Errorf("Expected empty parents map, got %d elements", len(parents))
	}
}

// TestFindParentComponents_Interface tests findParentComponents with interfaces
func TestFindParentComponents_Interface(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	parents := lc.findParentComponents(comp)
	if len(parents) != 0 {
		t.Errorf("Expected empty parents map for interface, got %d elements", len(parents))
	}
}

// TestFindParentComponents_Pointer tests findParentComponents with pointers
func TestFindParentComponents_Pointer(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	parents := lc.findParentComponents(comp)
	if len(parents) != 0 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

// TestFindParentComponents_Struct tests findParentComponents with structs
func TestFindParentComponents_Struct(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	testStruct := &TestStruct{Dep1: comp}
	parents := lc.findParentComponents(testStruct)
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

// TestDependencies_Empty tests Dependencies with empty component set
func TestDependencies_Empty(t *testing.T) {
	lc := setupTestLifecycle()
	deps := lc.Dependencies()
	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies, got %d elements", len(deps))
	}
}

// TestDependencies_NoDeps tests Dependencies with components without dependencies
func TestDependencies_NoDeps(t *testing.T) {
	lc := setupTestLifecycle()
	comp := &mockComponent{name: "test"}
	lc.Register(comp)

	deps := lc.Dependencies()
	if len(deps) != 1 {
		t.Errorf("Expected 1 component, got %d", len(deps))
	}
	if len(deps[comp]) != 0 {
		t.Errorf("Expected no dependencies, got %d", len(deps[comp]))
	}
}

// TestBuildCompToParents_Empty tests buildCompToParents with empty component set
func TestBuildCompToParents_Empty(t *testing.T) {
	lc := setupTestLifecycle()
	parents := lc.buildCompToParents()
	if len(parents) != 0 {
		t.Errorf("Expected empty parents map, got %d elements", len(parents))
	}
}

// TestBuildCompToChildren_Empty tests buildCompToChildren with empty parent graph
func TestBuildCompToChildren_Empty(t *testing.T) {
	lc := setupTestLifecycle()
	children := lc.buildCompToChildren(make(map[Component]map[Component]struct{}))
	if len(children) != 0 {
		t.Errorf("Expected empty children map, got %d elements", len(children))
	}
}

type sliceMockComponent struct {
	mockComponent
	arr []*mockComponent
}

// TestFindParentComponents_Slice tests findParentComponents with slices
func TestFindParentComponents_Slice(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	slice := &sliceMockComponent{arr: []*mockComponent{comp}}
	parents := lc.findParentComponents(slice)
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

type arrayMockComponent struct {
	mockComponent
	arr [1]*mockComponent
}

// TestFindParentComponents_Array tests findParentComponents with arrays
func TestFindParentComponents_Array(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	array := &arrayMockComponent{arr: [1]*mockComponent{comp}}
	parents := lc.findParentComponents(array)
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

type mapMockComponent struct {
	mockComponent
	m map[string]*mockComponent
}

// TestFindParentComponents_Map tests findParentComponents with maps
func TestFindParentComponents_Map(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	m := &mapMockComponent{m: map[string]*mockComponent{"test": comp}}
	parents := lc.findParentComponents(m)
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

// TestFindParentComponents_NestedStruct tests findParentComponents with nested structs
func TestFindParentComponents_NestedStruct(t *testing.T) {
	lc := setupTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	type InnerStruct struct {
		Comp *mockComponent
	}

	type OuterStruct struct {
		mockComponent
		Inner InnerStruct
	}

	outer := &OuterStruct{
		Inner: InnerStruct{Comp: comp},
	}

	parents := lc.findParentComponents(outer)
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

// TestFindParentComponents_MultipleDeps tests findParentComponents with multiple dependencies
func TestFindParentComponents_MultipleDeps(t *testing.T) {
	lc := setupTestLifecycle()

	comp1 := &mockComponent{name: "test1"}
	comp2 := &mockComponent{name: "test2"}
	lc.ptrToComp[reflect.ValueOf(comp1).Pointer()] = comp1
	lc.ptrToComp[reflect.ValueOf(comp2).Pointer()] = comp2

	testStruct := &TestStruct{Dep1: comp1, Dep2: comp2}
	parents := lc.findParentComponents(testStruct)
	if len(parents) != 2 {
		t.Errorf("Expected 2 parents, got %d", len(parents))
	}
}

// TestDependencies_WithDeps tests Dependencies with components that have dependencies
func TestDependencies_WithDeps(t *testing.T) {
	lc := setupTestLifecycle()
	comp1 := &mockComponent{name: "test1"}
	comp2 := &mockComponent{name: "test2"}
	lc.Register(comp1)
	lc.Register(comp2)
	lc.ptrToComp[reflect.ValueOf(comp1).Pointer()] = comp1
	testStruct := &TestStruct{Dep1: comp1}
	lc.Register(testStruct)
	deps := lc.Dependencies()
	if len(deps) != 3 {
		t.Errorf("Expected 3 components, got %d", len(deps))
	}
	if len(deps[testStruct]) != 1 {
		t.Errorf("Expected 1 dependency for testStruct, got %d", len(deps[testStruct]))
	}
}

// TestBuildCompToParents_WithDeps tests buildCompToParents with components that have dependencies
func TestBuildCompToParents_WithDeps(t *testing.T) {
	lc := setupTestLifecycle()
	comp1 := &mockComponent{name: "test1"}
	comp2 := &mockComponent{name: "test2"}
	lc.Register(comp1)
	lc.Register(comp2)
	lc.ptrToComp[reflect.ValueOf(comp1).Pointer()] = comp1
	testStruct := &TestStruct{Dep1: comp1}
	lc.Register(testStruct)
	parents := lc.buildCompToParents()
	if len(parents) != 3 {
		t.Errorf("Expected 1 component with parents, got %d", len(parents))
	}
	if len(parents[testStruct]) != 1 {
		t.Errorf("Expected 1 parent for testStruct, got %d", len(parents[testStruct]))
	}
}

// TestBuildCompToChildren_WithDeps tests buildCompToChildren with components that have dependencies
func TestBuildCompToChildren_WithDeps(t *testing.T) {
	lc := setupTestLifecycle()
	comp1 := &mockComponent{name: "test1"}
	comp2 := &mockComponent{name: "test2"}
	lc.Register(comp1)
	lc.Register(comp2)
	lc.ptrToComp[reflect.ValueOf(comp1).Pointer()] = comp1
	testStruct := &TestStruct{Dep1: comp1}
	lc.Register(testStruct)
	parents := lc.buildCompToParents()
	children := lc.buildCompToChildren(parents)
	if len(children) != 1 {
		t.Errorf("Expected 1 component with children, got %d", len(children))
	}
	if len(children[comp1]) != 1 {
		t.Errorf("Expected 1 child for comp1, got %d", len(children[comp1]))
	}
}

// TestFindParentComponents_CircularDeps tests findParentComponents with circular dependencies
func TestFindParentComponents_CircularDeps(t *testing.T) {
	lc := setupTestLifecycle()

	type CircularStruct struct {
		mockComponent
		Self *CircularStruct
	}

	circular := &CircularStruct{}
	circular.Self = circular

	parents := lc.findParentComponents(circular)
	if len(parents) != 0 {
		t.Errorf("Expected no parents for circular dependency, got %d", len(parents))
	}
}

// TestDependencies_ComplexGraph tests Dependencies with a complex dependency graph
func TestDependencies_ComplexGraph(t *testing.T) {
	lc := setupTestLifecycle()
	// Create components
	comp1 := &mockComponent{name: "test1"}
	comp2 := &mockComponent{name: "test2"}
	comp3 := &mockComponent{name: "test3"}
	lc.Register(comp1)
	lc.Register(comp2)
	lc.Register(comp3)
	lc.ptrToComp[reflect.ValueOf(comp1).Pointer()] = comp1
	lc.ptrToComp[reflect.ValueOf(comp2).Pointer()] = comp2
	lc.ptrToComp[reflect.ValueOf(comp3).Pointer()] = comp3
	// Create complex dependency structure
	complex := &ComplexStruct{
		Dep1: comp1,
		Dep2: comp2,
		Dep3: comp3,
	}
	lc.Register(complex)
	deps := lc.Dependencies()
	if len(deps) != 4 {
		t.Errorf("Expected 4 components, got %d", len(deps))
	}
	if len(deps[complex]) != 3 {
		t.Errorf("Expected 3 dependencies for complex struct, got %d", len(deps[complex]))
	}
}

// TestBuildCompToParents_CycleGraph tests cycle graph in buildCompToParents
func TestBuildCompToParents_CycleGraph(t *testing.T) {
	lc := setupTestLifecycle()
	comp1 := Register(lc, &mockComponent{name: "test1"})
	comp2 := Register(lc, &mockComponent{name: "test2"})
	rec1 := Register(lc, &RecStruct{Name: "rec1", Comp: comp1})
	rec2 := Register(lc, &RecStruct{Name: "rec2", Comp: comp2, Dep: rec1})
	rec1.Dep = rec2 // Create cycle
	assert.Panicsf(t, func() { lc.Dependencies() }, "Expected panic due to cycle in dependencies")
}
