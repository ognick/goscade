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

// IgnoreTagStruct is used to test the goscade:"ignore" field tag
type IgnoreTagStruct struct {
	Dep1 *mockComponent
	Dep2 *mockComponent `goscade:"ignore"`
}

func (s *IgnoreTagStruct) Run(ctx context.Context, readinessProbe func(cause error)) error {
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

// newTestLifecycle creates a new lifecycle for testing
func newTestLifecycle() *lifecycle {
	return NewLifecycle(&mockLogger{}).(*lifecycle)
}

// TestLink_RegistersComponentAndStoresDeps verifies Link registers the
// component (idempotent) and records the explicit struct deps.
func TestLink_RegistersComponentAndStoresDeps(t *testing.T) {
	lc := newTestLifecycle()

	comp := &mockComponent{name: "test"}
	type wiring struct{ X int }
	w := &wiring{X: 1}

	lc.Link(comp, w)

	if _, ok := lc.components[comp]; !ok {
		t.Fatalf("expected Link to register the component")
	}
	if len(lc.compToLinkedDeps[comp]) != 1 {
		t.Fatalf("expected 1 explicit dep, got %d", len(lc.compToLinkedDeps[comp]))
	}

	// Idempotent: calling Link again appends, does not duplicate the component.
	lc.Link(comp, w)
	if len(lc.components) != 1 {
		t.Fatalf("expected 1 component after second Link, got %d", len(lc.components))
	}
	if len(lc.compToLinkedDeps[comp]) != 2 {
		t.Fatalf("expected 2 explicit deps after second Link, got %d", len(lc.compToLinkedDeps[comp]))
	}
}

// TestLink_DiscoversComponentBehindStruct verifies that a component with no
// field reference to another component still gets it as a parent when an
// arbitrary struct that references it is linked explicitly.
func TestLink_DiscoversComponentBehindStruct(t *testing.T) {
	lc := newTestLifecycle()

	dep := Register(lc, &mockComponent{name: "dep"})

	// wiring is NOT a Component; it holds a reference to dep.
	type wiring struct {
		Comp *mockComponent
	}
	w := &wiring{Comp: dep}

	// root has no field referencing dep at all.
	root := &mockComponent{name: "root"}
	lc.Link(root, w)

	parents := lc.findParentComponents(root)
	if len(parents) != 1 {
		t.Fatalf("expected 1 parent discovered through the lens, got %d", len(parents))
	}
	if _, ok := parents[dep]; !ok {
		t.Fatalf("expected dep to be a parent of root")
	}
}

// TestLinkHelper_ReturnsComponentAndLinks verifies the package-level Link
// helper registers, links, and returns the same component.
func TestLinkHelper_ReturnsComponentAndLinks(t *testing.T) {
	lc := newTestLifecycle()

	dep := Register(lc, &mockComponent{name: "dep"})
	type wiring struct{ Comp *mockComponent }
	w := &wiring{Comp: dep}

	root := Link(lc, &mockComponent{name: "root"}, w)

	if root == nil {
		t.Fatalf("expected Link to return the component")
	}
	parents := lc.findParentComponents(root)
	if _, ok := parents[dep]; !ok {
		t.Fatalf("expected dep to be a parent of root via the helper")
	}
}

// TestLink_NoSelfParent verifies a lens that references the component itself
// does not produce a self-parent.
func TestLink_NoSelfParent(t *testing.T) {
	lc := newTestLifecycle()

	root := Register(lc, &mockComponent{name: "root"})
	type wiring struct{ Self *mockComponent }
	w := &wiring{Self: root}
	lc.Link(root, w)

	parents := lc.findParentComponents(root)
	if len(parents) != 0 {
		t.Fatalf("expected no self-parent, got %d parents", len(parents))
	}
}

// TestLink_EmptyAndNilDeps verifies Link with no deps or a nil dep is safe.
func TestLink_EmptyAndNilDeps(t *testing.T) {
	lc := newTestLifecycle()

	root := &mockComponent{name: "root"}
	lc.Link(root)      // no deps
	lc.Link(root, nil) // nil dep

	parents := lc.findParentComponents(root)
	if len(parents) != 0 {
		t.Fatalf("expected no parents, got %d", len(parents))
	}
}

// TestLink_MultipleComponentsBehindStruct verifies all components reachable
// inside the lens (incl. nested structs) become parents.
func TestLink_MultipleComponentsBehindStruct(t *testing.T) {
	lc := newTestLifecycle()

	a := Register(lc, &mockComponent{name: "a"})
	b := Register(lc, &mockComponent{name: "b"})

	type inner struct{ B *mockComponent }
	type wiring struct {
		A     *mockComponent
		Inner inner
	}
	w := &wiring{A: a, Inner: inner{B: b}}

	root := &mockComponent{name: "root"}
	lc.Link(root, w)

	parents := lc.findParentComponents(root)
	if len(parents) != 2 {
		t.Fatalf("expected 2 parents, got %d", len(parents))
	}
	if _, ok := parents[a]; !ok {
		t.Fatalf("expected a to be a parent")
	}
	if _, ok := parents[b]; !ok {
		t.Fatalf("expected b to be a parent")
	}
}

// TestLink_CycleViaLensPanics verifies a cycle introduced through a lens is
// detected by Dependencies().
func TestLink_CycleViaLensPanics(t *testing.T) {
	lc := newTestLifecycle()

	a := Register(lc, &mockComponent{name: "a"})
	b := Register(lc, &mockComponent{name: "b"})

	// a depends on b, and b depends on a — both via lenses → cycle.
	type wiringA struct{ B *mockComponent }
	type wiringB struct{ A *mockComponent }
	lc.Link(a, &wiringA{B: b})
	lc.Link(b, &wiringB{A: a})

	assert.Panicsf(t, func() { lc.Dependencies() }, "expected panic due to cycle via lenses")
}

// TestFindParentComponents_Empty tests findParentComponents with empty values
func TestFindParentComponents_Empty(t *testing.T) {
	lc := newTestLifecycle()
	parents := lc.findParentComponents(nil)
	if len(parents) != 0 {
		t.Errorf("Expected empty parents map, got %d elements", len(parents))
	}
}

// TestFindParentComponents_Interface tests findParentComponents with interfaces
func TestFindParentComponents_Interface(t *testing.T) {
	lc := newTestLifecycle()

	comp := &mockComponent{name: "test"}
	parents := lc.findParentComponents(comp)
	if len(parents) != 0 {
		t.Errorf("Expected empty parents map for interface, got %d elements", len(parents))
	}
}

// TestFindParentComponents_Pointer tests findParentComponents with pointers
func TestFindParentComponents_Pointer(t *testing.T) {
	lc := newTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	parents := lc.findParentComponents(comp)
	if len(parents) != 0 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

// TestFindParentComponents_Struct tests findParentComponents with structs
func TestFindParentComponents_Struct(t *testing.T) {
	lc := newTestLifecycle()

	comp := &mockComponent{name: "test"}
	lc.ptrToComp[reflect.ValueOf(comp).Pointer()] = comp

	testStruct := &TestStruct{Dep1: comp}
	parents := lc.findParentComponents(testStruct)
	if len(parents) != 1 {
		t.Errorf("Expected 1 parent, got %d", len(parents))
	}
}

// TestFindParentComponents_IgnoreTag tests that fields tagged goscade:"ignore" are skipped
func TestFindParentComponents_IgnoreTag(t *testing.T) {
	lc := newTestLifecycle()

	dep1 := &mockComponent{name: "dep1"}
	dep2 := &mockComponent{name: "dep2"}
	lc.ptrToComp[reflect.ValueOf(dep1).Pointer()] = dep1
	lc.ptrToComp[reflect.ValueOf(dep2).Pointer()] = dep2

	testStruct := &IgnoreTagStruct{Dep1: dep1, Dep2: dep2}
	parents := lc.findParentComponents(testStruct)
	assert.Len(t, parents, 1)
	assert.Contains(t, parents, Component(dep1))
	assert.NotContains(t, parents, Component(dep2))
}

// TestDependencies_Empty tests Dependencies with empty component set
func TestDependencies_Empty(t *testing.T) {
	lc := newTestLifecycle()
	deps := lc.Dependencies()
	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies, got %d elements", len(deps))
	}
}

// TestDependencies_NoDeps tests Dependencies with components without dependencies
func TestDependencies_NoDeps(t *testing.T) {
	lc := newTestLifecycle()
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
	lc := newTestLifecycle()
	parents := lc.buildCompToParents()
	if len(parents) != 0 {
		t.Errorf("Expected empty parents map, got %d elements", len(parents))
	}
}

// TestBuildCompToChildren_Empty tests buildCompToChildren with empty parent graph
func TestBuildCompToChildren_Empty(t *testing.T) {
	lc := newTestLifecycle()
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
	lc := newTestLifecycle()

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
	lc := newTestLifecycle()

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
	lc := newTestLifecycle()

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
	lc := newTestLifecycle()

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
	lc := newTestLifecycle()

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
	lc := newTestLifecycle()
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
	lc := newTestLifecycle()
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
	lc := newTestLifecycle()
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
	lc := newTestLifecycle()

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
	lc := newTestLifecycle()
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
	lc := newTestLifecycle()
	comp1 := Register(lc, &mockComponent{name: "test1"})
	comp2 := Register(lc, &mockComponent{name: "test2"})
	rec1 := Register(lc, &RecStruct{Name: "rec1", Comp: comp1})
	rec2 := Register(lc, &RecStruct{Name: "rec2", Comp: comp2, Dep: rec1})
	rec1.Dep = rec2 // Create cycle
	assert.Panicsf(t, func() { lc.Dependencies() }, "Expected panic due to cycle in dependencies")
}
