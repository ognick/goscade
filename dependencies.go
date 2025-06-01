package goscade

import (
	"reflect"
)

func (lc *lifecycle) findParentComponents(
	val reflect.Value,
	visited map[uintptr]struct{},
	parents map[Component]struct{},
	depth uint64,
) {
	if val.Kind() == reflect.Interface {
		val = val.Elem()
	}

	if val.Kind() == reflect.Pointer {
		ptr := val.Pointer()
		if _, seen := visited[ptr]; seen {
			return
		}

		visited[ptr] = struct{}{}
		if depth > 0 {
			if comp, ok := lc.ptrToComp[ptr]; ok {
				parents[comp] = struct{}{}
				return
			}
		}
	}

	switch val.Kind() {
	case reflect.Struct:
		for i := 0; i < val.NumField(); i++ {
			field := val.Field(i)
			lc.findParentComponents(field, visited, parents, depth+1)
		}

	case reflect.Interface, reflect.Pointer:
		lc.findParentComponents(val.Elem(), visited, parents, depth+1)

	case reflect.Slice, reflect.Array:
		for i := 0; i < val.Len(); i++ {
			lc.findParentComponents(val.Index(i), visited, parents, depth+1)
		}

	case reflect.Map:
		iter := val.MapRange()
		for iter.Next() {
			lc.findParentComponents(iter.Key(), visited, parents, depth+1)
			lc.findParentComponents(iter.Value(), visited, parents, depth+1)
		}
	default:
		return
	}
}

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

func (lc *lifecycle) buildCompToParents() map[Component]map[Component]struct{} {
	compToParents := make(map[Component]map[Component]struct{})
	for comp := range lc.components {
		parents := make(map[Component]struct{})
		root := reflect.ValueOf(comp)
		lc.findParentComponents(root, make(map[uintptr]struct{}), parents, 0)
		if len(parents) > 0 {
			compToParents[comp] = parents
		}
	}
	return compToParents
}

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
