package goscade

func RegisterComponent[T Component](lc Lifecycle, component T) T {
	lc.Register(component)
	return component
}
