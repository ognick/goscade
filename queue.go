package goscade

// Queue defines a generic queue interface for managing items of type T.
// It provides basic operations for adding, removing, and checking the queue state.
type Queue[T any] interface {
	// Push adds an item to the queue.
	Push(item T)

	// Pop removes and returns an item from the queue.
	// Returns the item and true if successful, or zero value and false if empty.
	Pop() (T, bool)

	// IsEmpty returns true if the queue contains no items.
	IsEmpty() bool
}

// FIFOQueue implements a First-In-First-Out queue using a slice.
// Items are added to the end and removed from the beginning.
type FIFOQueue[T any] struct {
	items []T
}

// Push adds an item to the end of the FIFO queue.
func (q *FIFOQueue[T]) Push(item T) {
	q.items = append(q.items, item)
}

// Pop removes and returns the first item from the FIFO queue.
// Returns the item and true if successful, or zero value and false if empty.
func (q *FIFOQueue[T]) Pop() (T, bool) {
	if q.IsEmpty() {
		var zero T
		return zero, false
	}
	x := q.items[0]
	q.items = q.items[1:]
	return x, true
}

// IsEmpty returns true if the FIFO queue contains no items.
func (q *FIFOQueue[T]) IsEmpty() bool {
	return len(q.items) == 0
}

// LIFOQueue implements a Last-In-First-Out queue (stack) using a slice.
// Items are added to the end and removed from the end.
type LIFOQueue[T any] struct {
	items []T
}

// Push adds an item to the end of the LIFO queue.
func (q *LIFOQueue[T]) Push(item T) {
	q.items = append(q.items, item)
}

// Pop removes and returns the last item from the LIFO queue.
// Returns the item and true if successful, or zero value and false if empty.
func (q *LIFOQueue[T]) Pop() (T, bool) {
	if q.IsEmpty() {
		var zero T
		return zero, false
	}
	last := len(q.items) - 1
	x := q.items[last]
	q.items = q.items[:last]
	return x, true
}

// IsEmpty returns true if the LIFO queue contains no items.
func (q *LIFOQueue[T]) IsEmpty() bool {
	return len(q.items) == 0
}
