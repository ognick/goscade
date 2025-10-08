package goscade

// Register is a convenience function that registers a component with the lifecycle
// manager and returns the same component. This allows for fluent-style registration
// and makes it easier to chain component creation and registration.
//
// Example:
//
//	server := Register(lc, NewServer(handler))
//	db := Register(lc, NewDatabase(config))
//	cache := Register(lc, NewCache(), db) // explicit dependency
//
// This is equivalent to:
//
//	server := NewServer(handler)
//	lc.Register(server)
//	db := NewDatabase(config)
//	lc.Register(db)
//	cache := NewCache()
//	lc.Register(cache, db) // explicit dependency
func Register[T Component](lc Lifecycle, component T, implicitDeps ...Component) T {
	lc.Register(component, implicitDeps...)
	return component
}
