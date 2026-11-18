package goscade

import (
	"fmt"
	"reflect"
	"sync"
)

// ptrKey identifies a pointer by its static type and address. The address
// alone is not enough: a pointer to a struct and a pointer to its first field
// are equal as numbers but point to different things.
type ptrKey struct {
	typ  reflect.Type
	addr uintptr
}

// ptrKeyOf returns the key of a pointer value.
func ptrKeyOf(val reflect.Value) ptrKey {
	return ptrKey{typ: val.Type(), addr: val.Pointer()}
}

// containerKey identifies a map or a slice that has already been walked, so a
// container that (indirectly) contains itself is not walked forever.
type containerKey struct {
	typ  reflect.Type
	addr uintptr
	len  int
}

// findParentComponents performs a breadth-first search traversal of a component's structure
// to find all parent components it depends on. It uses reflection to examine fields,
// slices, arrays, maps, and nested structures.
//
// Values whose type can never lead to a component (see lifecycle.traversable) are
// skipped as a whole instead of being walked element by element.
//
// Parameters:
//   - root: Component to examine
//
// Returns:
//   - parents: map to collect found parent components
func (lc *lifecycle) findParentComponents(root Component) map[Component]struct{} {
	visited := make(map[ptrKey]struct{})
	visitedContainers := make(map[containerKey]struct{})
	queue := fifoQueue[reflect.Value]{}
	parents := make(map[Component]struct{})
	for dep := range lc.compToImplicitDeps[root] {
		parents[dep] = struct{}{}
	}

	// The root itself is never its own parent.
	var rootKey ptrKey
	if rootVal := reflect.ValueOf(root); rootVal.IsValid() {
		if rootVal.Kind() == reflect.Pointer {
			rootKey = ptrKeyOf(rootVal)
		}
		queue.Push(rootVal)
	}

	// Explicit struct deps (from Link) are seeded as extra traversal roots so
	// the BFS below walks them exactly as if they were fields of root.
	for _, dep := range lc.compToLinkedDeps[root] {
		queue.Push(reflect.ValueOf(dep))
	}

	// parentAt records the component registered under key, if there is one.
	parentAt := func(key ptrKey) bool {
		if key == rootKey {
			return false
		}
		comp, ok := lc.ptrToComp[key]
		if ok {
			parents[comp] = struct{}{}
		}
		return ok
	}

	for !queue.IsEmpty() {
		val, _ := queue.Pop()
		if val.Kind() == reflect.Interface {
			val = val.Elem()
		}

		// One type-level check covers every kind: nil interfaces, scalars,
		// and containers that hold nothing but scalars stop here.
		if !val.IsValid() || !lc.traversable(val.Type()) {
			continue
		}

		switch val.Kind() {
		case reflect.Pointer:
			if val.IsNil() {
				continue
			}

			key := ptrKeyOf(val)
			if _, seen := visited[key]; seen {
				continue
			}

			visited[key] = struct{}{}
			if parentAt(key) {
				continue
			}

			queue.Push(val.Elem())

		case reflect.Struct:
			// A component stored by value is found by its address.
			if val.CanAddr() {
				key := ptrKey{typ: reflect.PointerTo(val.Type()), addr: val.UnsafeAddr()}
				if parentAt(key) {
					continue
				}
			}

			for _, i := range lc.traversableFields(val.Type()) {
				queue.Push(val.Field(i))
			}

		case reflect.Array:
			for i := 0; i < val.Len(); i++ {
				queue.Push(val.Index(i))
			}

		case reflect.Slice:
			if val.Len() == 0 {
				continue
			}

			key := containerKey{typ: val.Type(), addr: val.Pointer(), len: val.Len()}
			if _, seen := visitedContainers[key]; seen {
				continue
			}

			visitedContainers[key] = struct{}{}
			for i := 0; i < val.Len(); i++ {
				queue.Push(val.Index(i))
			}

		case reflect.Map:
			if val.Len() == 0 {
				continue
			}

			key := containerKey{typ: val.Type(), addr: val.Pointer()}
			if _, seen := visitedContainers[key]; seen {
				continue
			}

			visitedContainers[key] = struct{}{}
			t := val.Type()
			walkKeys, walkElems := lc.traversable(t.Key()), lc.traversable(t.Elem())
			iter := val.MapRange()
			for iter.Next() {
				if walkKeys {
					queue.Push(iter.Key())
				}
				if walkElems {
					queue.Push(iter.Value())
				}
			}
		}
	}

	return parents
}

// ignoredField reports whether a struct field is excluded from dependency
// detection with the `goscade:"ignore"` tag.
func ignoredField(f reflect.StructField) bool {
	return f.Tag.Get("goscade") == "ignore"
}

// typeFacts caches what the traversal knows about types. The facts depend on
// which component types are registered, so they belong to one lifecycle and
// are dropped whenever a new component type is registered. The caches only
// grow with the number of distinct types met, which is bounded by the program.
type typeFacts struct {
	traversable sync.Map // map[reflect.Type]bool
	fields      sync.Map // map[reflect.Type][]int
}

// traversable reports whether the BFS in findParentComponents can reach a
// component through a value of type t, i.e. whether t is, or contains, a
// pointer, an interface, or a registered component stored by value.
//
// Types made only of scalars (bytes, ints, strings, ...) are not traversable,
// so arrays, slices, maps and struct fields of such types are skipped as a
// whole. Channels, functions and unsafe pointers are not traversable either:
// they do hold pointers, but the BFS does not look inside them.
func (lc *lifecycle) traversable(t reflect.Type) bool {
	if v, ok := lc.types.traversable.Load(t); ok {
		return v.(bool)
	}

	seen := make(map[reflect.Type]struct{})
	res := lc.reachesComponent(t, seen)
	if res {
		lc.types.traversable.Store(t, true)
		return true
	}

	// Nothing reachable from t can lead to a component, so the same holds
	// for every type met on the way.
	for s := range seen {
		lc.types.traversable.Store(s, false)
	}

	return false
}

// reachesComponent walks the type graph below t looking for a type that can
// lead to a component. Types already in seen are not entered again: that is
// what keeps self-referential types such as
// `type Node struct{ Children []Node }` from recursing forever, and it is safe
// because a type in seen either is still being examined or has already been
// found to lead nowhere.
func (lc *lifecycle) reachesComponent(t reflect.Type, seen map[reflect.Type]struct{}) bool {
	if v, ok := lc.types.traversable.Load(t); ok {
		return v.(bool)
	}

	if _, ok := seen[t]; ok {
		return false
	}

	seen[t] = struct{}{}
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface:
		return true
	case reflect.Array:
		return t.Len() > 0 && lc.reachesComponent(t.Elem(), seen)
	case reflect.Slice:
		return lc.reachesComponent(t.Elem(), seen)
	case reflect.Map:
		return lc.reachesComponent(t.Key(), seen) || lc.reachesComponent(t.Elem(), seen)
	case reflect.Struct:
		// A registered component may be stored by value.
		if _, ok := lc.compElemTypes[t]; ok {
			return true
		}

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if !ignoredField(f) && lc.reachesComponent(f.Type, seen) {
				return true
			}
		}
	}

	return false
}

// traversableFields returns the indices of the fields of struct type t that
// the BFS has to walk: not ignored by tag and of a traversable type. The list
// is computed once per type.
func (lc *lifecycle) traversableFields(t reflect.Type) []int {
	if v, ok := lc.types.fields.Load(t); ok {
		return v.([]int)
	}

	var fields []int
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !ignoredField(f) && lc.traversable(f.Type) {
			fields = append(fields, i)
		}
	}

	lc.types.fields.Store(t, fields)
	return fields
}

// findCircularDependencies finds and optionally removes components that are part of circular
// dependency chains from the component-to-parents mapping. It uses BFS
// traversal to detect cycles and removes components that would create
// circular dependencies.
//
// This function modifies the compToParents map in-place by removing
// components that are part of circular dependency chains.
func findCircularDependencies(
	compToParents map[Component]map[Component]struct{},
	removeCircularDependency bool,
) {
	for root := range compToParents {
		queue := fifoQueue[Component]{}
		queue.Push(root)
		for !queue.IsEmpty() {
			node, _ := queue.Pop()
			for parent := range compToParents[node] {
				if parent == root {
					if removeCircularDependency {
						delete(compToParents, node)
						continue
					}

					panic(fmt.Sprintf("circular dependency detected %s <-> %s",
						reflect.ValueOf(root).Type().String(),
						reflect.ValueOf(node).Type().String(),
					))
				}

				queue.Push(parent)
			}
		}
	}
}

// Dependencies returns a map of each component to its list of dependencies.
// The returned map shows the dependency graph where each component is mapped
// to a slice of components it depends on (its parents in the dependency tree).
//
// Components without dependencies will have an empty slice.
// This method is useful for debugging and understanding the component graph.
func (lc *lifecycle) Dependencies() map[Component][]Component {
	deps := make(map[Component][]Component)
	compToParents := lc.buildCompToParents()
	for comp := range lc.components {
		parents, ok := compToParents[comp]
		if !ok {
			deps[comp] = make([]Component, 0)
			continue
		}

		deps[comp] = make([]Component, 0, len(parents))
		for parent := range parents {
			deps[comp] = append(deps[comp], parent)
		}
	}
	return deps
}

// buildCompToParents builds a mapping from each component to its parent components.
// It uses concurrent goroutines to analyze component dependencies in parallel
// for better performance with large component graphs.
//
// The function returns a map where each component is mapped to a set of
// components it depends on. If circular dependency detection is enabled,
// components that would create cycles are removed from the mapping.
//
// This is an internal method used by the lifecycle management system.
func (lc *lifecycle) buildCompToParents() map[Component]map[Component]struct{} {
	compToParents := make(map[Component]map[Component]struct{})
	var wg sync.WaitGroup
	for comp := range lc.components {
		wg.Add(1)
		go func() {
			defer wg.Done()
			parents := lc.findParentComponents(comp)
			lc.mu.Lock()
			compToParents[comp] = parents
			lc.mu.Unlock()
		}()
	}

	wg.Wait()
	findCircularDependencies(compToParents, lc.ignoreCircularDependency)
	return compToParents
}

// buildCompToChildren builds a mapping from each component to its child components.
// It takes the parent-to-child relationships from compToParents and inverts them
// to create a child-to-parent mapping.
//
// This is useful for understanding which components depend on a given component
// and for implementing proper shutdown order (children should be shut down before parents).
//
// This is an internal method used by the lifecycle management system.
func (lc *lifecycle) buildCompToChildren(
	compToParents map[Component]map[Component]struct{},
) map[Component]map[Component]struct{} {
	compToChildren := make(map[Component]map[Component]struct{})
	addChild := func(comp Component, child Component) {
		children, ok := compToChildren[comp]
		if !ok {
			children = make(map[Component]struct{})
			compToChildren[comp] = children
		}

		children[child] = struct{}{}
	}

	for child, parents := range compToParents {
		for p := range parents {
			addChild(p, child)
		}
	}

	return compToChildren
}
