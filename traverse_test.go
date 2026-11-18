package goscade

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// bigBlob is a component whose only payload is pointer-free data.
// The BFS must not walk its elements one by one.
type bigBlob struct {
	Buf    [1 << 20]byte
	Nested [1024]struct {
		A int
		B [64]byte
	}
	Bytes []byte
	Ints  map[int]int
}

func (b *bigBlob) Run(_ context.Context, _ func(error)) error { return nil }

// blobWithDep has a real dependency next to a big pointer-free array.
type blobWithDep struct {
	Buf [1 << 20]byte
	Dep *mockComponent
	Arr [2]*mockComponent
}

func (b *blobWithDep) Run(_ context.Context, _ func(error)) error { return nil }

func TestTraversable(t *testing.T) {
	type flat struct {
		A int
		B [16]byte
		C struct{ X, Y float64 }
	}
	type withPtr struct {
		A int
		P *int
	}
	type ignoredPtr struct {
		A int
		P *mockComponent `goscade:"ignore"`
	}
	type rec struct {
		Next *rec
	}
	// Value-recursive types: they refer to themselves without a pointer.
	type node struct {
		Name     string
		Children []node
	}
	type nodeMap map[string]nodeMap
	type nodeSlice []nodeSlice
	type treeWithPtr struct {
		Kids []treeWithPtr
		P    *int
	}

	cases := []struct {
		name string
		v    any
		want bool
	}{
		{"byte", byte(0), false},
		{"int", 0, false},
		{"string", "", false},
		{"byte array", [1 << 10]byte{}, false},
		{"byte slice", []byte{}, false},
		{"flat struct", flat{}, false},
		{"array of flat struct", [8]flat{}, false},
		{"int map", map[int]int{}, false},
		{"empty pointer array", [0]*int{}, false},
		{"ignored pointer field", ignoredPtr{}, false},
		{"pointer", (*int)(nil), true},
		{"struct with pointer", withPtr{}, true},
		{"array of struct with pointer", [4]withPtr{}, true},
		{"slice of pointers", []*int{}, true},
		{"map with pointer value", map[int]*int{}, true},
		{"map with pointer key", map[*int]int{}, true},
		{"interface slice", []any{}, true},
		{"recursive struct", rec{}, true},
		{"value-recursive struct", node{}, false},
		{"value-recursive map", nodeMap{}, false},
		{"value-recursive slice", nodeSlice{}, false},
		{"value-recursive struct with pointer", treeWithPtr{}, true},
		{"chan is not walked", make(chan *int), false},
		{"func is not walked", func() {}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, newTestLifecycle().traversable(reflect.TypeOf(c.v)))
		})
	}
}

func TestFindParentComponents_SkipsPointerFreeData(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	arrDep := &mockComponent{name: "arr"}
	comp := &blobWithDep{Dep: dep, Arr: [2]*mockComponent{nil, arrDep}}
	lc.Register(dep)
	lc.Register(arrDep)
	lc.Register(comp)

	parents := lc.findParentComponents(comp)
	assert.Len(t, parents, 2)
	assert.Contains(t, parents, Component(dep))
	assert.Contains(t, parents, Component(arrDep))
}

func TestFindParentComponents_BigPointerFreeComponentIsCheap(t *testing.T) {
	lc := newTestLifecycle()
	comp := &bigBlob{Bytes: make([]byte, 1<<20), Ints: map[int]int{}}
	for i := 0; i < 1000; i++ {
		comp.Ints[i] = i
	}
	lc.Register(comp)

	allocs := testing.AllocsPerRun(5, func() {
		lc.findParentComponents(comp)
	})
	// Walking every byte would allocate millions of times.
	assert.Less(t, allocs, 100.0)
}

func BenchmarkFindParentComponents_BigBlob(b *testing.B) {
	lc := newTestLifecycle()
	comp := &bigBlob{Bytes: make([]byte, 1<<20)}
	lc.Register(comp)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lc.findParentComponents(comp)
	}
}

// Mutually recursive types. The answer must not depend on which one is
// asked about first.
type mutA struct {
	X []mutB
	P *int
}
type mutB struct{ Y []mutA }

func TestTraversable_RegisteredComponentByValue(t *testing.T) {
	lc := newTestLifecycle()
	typ := reflect.TypeOf([4]innerComp{})
	assert.False(t, lc.traversable(typ))

	// Registering the type must drop the cached "not traversable" answer.
	lc.Register(&innerComp{})
	assert.True(t, lc.traversable(typ))
}

func TestTraversable_MutualRecursion(t *testing.T) {
	// Owner of the pointer first.
	lc := newTestLifecycle()
	assert.True(t, lc.traversable(reflect.TypeOf(mutA{})))
	assert.True(t, lc.traversable(reflect.TypeOf(mutB{})))
	// The other one first, with empty caches.
	lc = newTestLifecycle()
	assert.True(t, lc.traversable(reflect.TypeOf(mutB{})))
	assert.True(t, lc.traversable(reflect.TypeOf(mutA{})))
}

// treeNode refers to itself by value; treeComp keeps a tree of them.
type treeNode struct {
	Name     string
	Children []treeNode
}

type treeComp struct {
	Root treeNode
	Dep  *mockComponent
}

func (c *treeComp) Run(_ context.Context, _ func(error)) error { return nil }

func TestFindParentComponents_ValueRecursiveType(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	comp := &treeComp{
		Root: treeNode{Name: "a", Children: []treeNode{{Name: "b"}}},
		Dep:  dep,
	}
	lc.Register(dep)
	lc.Register(comp)

	parents := lc.findParentComponents(comp)
	assert.Len(t, parents, 1)
	assert.Contains(t, parents, Component(dep))
}

// cyclicComp holds containers that contain themselves.
type cyclicComp struct {
	M   map[string]any
	S   []any
	Dep *mockComponent
}

func (c *cyclicComp) Run(_ context.Context, _ func(error)) error { return nil }

// findParentsWithTimeout fails the test instead of hanging forever.
func findParentsWithTimeout(t *testing.T, lc *lifecycle, comp Component) map[Component]struct{} {
	t.Helper()
	done := make(chan map[Component]struct{}, 1)
	go func() { done <- lc.findParentComponents(comp) }()
	select {
	case parents := <-done:
		return parents
	case <-time.After(5 * time.Second):
		t.Fatal("findParentComponents did not finish: value cycle is walked forever")
		return nil
	}
}

func TestFindParentComponents_SelfContainingMap(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	m := map[string]any{}
	m["self"] = m
	comp := &cyclicComp{M: m, Dep: dep}
	lc.Register(dep)
	lc.Register(comp)

	parents := findParentsWithTimeout(t, lc, comp)
	assert.Len(t, parents, 1)
	assert.Contains(t, parents, Component(dep))
}

func TestFindParentComponents_SelfContainingSlice(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	s := []any{nil, dep}
	s[0] = s
	comp := &cyclicComp{S: s}
	lc.Register(dep)
	lc.Register(comp)

	parents := findParentsWithTimeout(t, lc, comp)
	assert.Len(t, parents, 1)
	assert.Contains(t, parents, Component(dep))
}

// innerComp is a component stored BY VALUE inside outer structs.
type innerComp struct{ name string }

func (c *innerComp) Run(_ context.Context, _ func(error)) error { return nil }

// outerFirst has the component as its first field, so &outer and
// &outer.In are the same address with different types.
type outerFirst struct {
	In  innerComp
	Dep *mockComponent
}

// outerSecond has the component at a non-zero offset.
type outerSecond struct {
	Pad int
	In  innerComp
	Dep *mockComponent
}

type holderFirst struct{ O *outerFirst }

func (c *holderFirst) Run(_ context.Context, _ func(error)) error { return nil }

type holderSecond struct{ O *outerSecond }

func (c *holderSecond) Run(_ context.Context, _ func(error)) error { return nil }

func TestFindParentComponents_SameAddressDifferentType(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	o := &outerFirst{Dep: dep}
	comp := &holderFirst{O: o}
	lc.Register(dep)
	lc.Register(&o.In)
	lc.Register(comp)

	// *outerFirst must not be mistaken for the *innerComp at the same
	// address: the walk has to go on and find Dep too.
	parents := lc.findParentComponents(comp)
	assert.Len(t, parents, 2)
	assert.Contains(t, parents, Component(dep))
	assert.Contains(t, parents, Component(&o.In))
}

func TestFindParentComponents_ComponentEmbeddedByValue(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	o := &outerSecond{Dep: dep}
	comp := &holderSecond{O: o}
	lc.Register(dep)
	lc.Register(&o.In)
	lc.Register(comp)

	parents := lc.findParentComponents(comp)
	assert.Len(t, parents, 2)
	assert.Contains(t, parents, Component(dep))
	assert.Contains(t, parents, Component(&o.In))
}

// outerComp is itself a component AND holds another one as first field.
type outerComp struct {
	In  innerComp
	Dep *mockComponent
}

func (c *outerComp) Run(_ context.Context, _ func(error)) error { return nil }

type holderBoth struct {
	O  *outerComp
	In *innerComp
}

func (c *holderBoth) Run(_ context.Context, _ func(error)) error { return nil }

func TestFindParentComponents_TwoComponentsAtSameAddress(t *testing.T) {
	lc := newTestLifecycle()
	dep := &mockComponent{name: "dep"}
	o := &outerComp{Dep: dep}
	comp := &holderBoth{O: o, In: &o.In}
	lc.Register(dep)
	lc.Register(o)
	lc.Register(&o.In)
	lc.Register(comp)

	// Both live at the same address; neither may hide the other.
	parents := lc.findParentComponents(comp)
	assert.Len(t, parents, 2)
	assert.Contains(t, parents, Component(o))
	assert.Contains(t, parents, Component(&o.In))

	// The outer one depends on the component it holds by value, not on itself.
	outerParents := lc.findParentComponents(o)
	assert.Len(t, outerParents, 2)
	assert.Contains(t, outerParents, Component(&o.In))
	assert.Contains(t, outerParents, Component(dep))
}

// boxedBlob hides pointer-free data behind an interface.
type boxedBlob struct {
	Box any
	Ptr *[1 << 20]byte
}

func (c *boxedBlob) Run(_ context.Context, _ func(error)) error { return nil }

func TestFindParentComponents_BoxedPointerFreeDataIsCheap(t *testing.T) {
	lc := newTestLifecycle()
	comp := &boxedBlob{Box: [1 << 20]byte{}, Ptr: new([1 << 20]byte)}
	lc.Register(comp)

	allocs := testing.AllocsPerRun(5, func() {
		lc.findParentComponents(comp)
	})
	assert.Less(t, allocs, 100.0)
}
