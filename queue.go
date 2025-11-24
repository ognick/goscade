package goscade

// queue defines a generic queue interface for managing items of type T.
// It provides basic operations for adding, removing, and checking the queue state.
type queue[T any] interface {
	// Push adds an item to the queue.
	Push(item T)

	// Pop removes and returns an item from the queue.
	// Returns the item and true if successful, or zero value and false if empty.
	Pop() (T, bool)

	// IsEmpty returns true if the queue contains no items.
	IsEmpty() bool
}

// fifoQueue implements a First-In-First-Out queue using a slice.
// Items are added to the end and removed from the beginning.
type fifoQueue[T any] struct {
	items []T
}

// Push adds an item to the end of the FIFO queue.
func (q *fifoQueue[T]) Push(item T) {
	q.items = append(q.items, item)
}

// Pop removes and returns the first item from the FIFO queue.
// Returns the item and true if successful, or zero value and false if empty.
func (q *fifoQueue[T]) Pop() (T, bool) {
	if q.IsEmpty() {
		var zero T
		return zero, false
	}
	x := q.items[0]
	q.items = q.items[1:]
	return x, true
}

// IsEmpty returns true if the FIFO queue contains no items.
func (q *fifoQueue[T]) IsEmpty() bool {
	return len(q.items) == 0
}

// lifoQueue implements a Last-In-First-Out queue (stack) using a slice.
// Items are added to the end and removed from the end.
type lifoQueue[T any] struct {
	items []T
}

// Push adds an item to the end of the LIFO queue.
func (q *lifoQueue[T]) Push(item T) {
	q.items = append(q.items, item)
}

// Pop removes and returns the last item from the LIFO queue.
// Returns the item and true if successful, or zero value and false if empty.
func (q *lifoQueue[T]) Pop() (T, bool) {
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
func (q *lifoQueue[T]) IsEmpty() bool {
	return len(q.items) == 0
}
