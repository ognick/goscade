package goscade

import (
	"fmt"
	"reflect"
	"sync"
)

// findParentComponents performs a breadth-first search traversal of a component's structure
// to find all parent components it depends on. It uses reflection to examine fields,
// slices, arrays, maps, and nested structures.
//
// Parameters:
//   - root: Component to examine
//
// Returns:
//   - parents: map to collect found parent components
func (lc *lifecycle) findParentComponents(root Component) map[Component]struct{} {
	visited := make(map[uintptr]struct{})
	queue := fifoQueue[reflect.Value]{}
	queue.Push(reflect.ValueOf(root))
	parents := make(map[Component]struct{})
	for dep := range lc.compToImplicitDeps[root] {
		parents[dep] = struct{}{}
	}

	var initialized bool
	for !queue.IsEmpty() {
		val, _ := queue.Pop()
		if val.Kind() == reflect.Interface {
			val = val.Elem()
		}

		if val.Kind() == reflect.Pointer {
			ptr := val.Pointer()
			if _, seen := visited[ptr]; seen {
				continue
			}

			visited[ptr] = struct{}{}
			if initialized {
				if comp, ok := lc.ptrToComp[ptr]; ok {
					parents[comp] = struct{}{}
					continue
				}
			}
			initialized = true
		}

		switch val.Kind() {
		case reflect.Struct:
			for i := 0; i < val.NumField(); i++ {
				queue.Push(val.Field(i))
			}

		case reflect.Interface, reflect.Pointer:
			queue.Push(val.Elem())

		case reflect.Slice, reflect.Array:
			for i := 0; i < val.Len(); i++ {
				queue.Push(val.Index(i))
			}

		case reflect.Map:
			iter := val.MapRange()
			for iter.Next() {
				queue.Push(iter.Key())
				queue.Push(iter.Value())
			}
		default:
			continue
		}
	}

	return parents
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
