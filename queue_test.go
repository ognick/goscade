package goscade

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFIFOQueue_Push tests adding items to the end of a FIFO queue.
func TestFIFOQueue_Push(t *testing.T) {
	queue := &FIFOQueue[int]{}

	queue.Push(1)
	queue.Push(2)
	queue.Push(3)

	assert.False(t, queue.IsEmpty())

	// Check that items are popped in FIFO order
	item, ok := queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 1, item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 2, item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 3, item)

	assert.True(t, queue.IsEmpty())
}

// TestFIFOQueue_Pop tests removing items from a FIFO queue in correct order.
func TestFIFOQueue_Pop(t *testing.T) {
	queue := &FIFOQueue[string]{}

	// Test empty queue
	_, ok := queue.Pop()
	assert.False(t, ok)

	// Add items
	queue.Push("first")
	queue.Push("second")
	queue.Push("third")

	// Pop and verify order
	item, ok := queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, "first", item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, "second", item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, "third", item)

	// Queue should be empty now
	_, ok = queue.Pop()
	assert.False(t, ok)
}

// TestFIFOQueue_IsEmpty tests the IsEmpty method of a FIFO queue.
func TestFIFOQueue_IsEmpty(t *testing.T) {
	queue := &FIFOQueue[int]{}

	assert.True(t, queue.IsEmpty())

	queue.Push(1)
	assert.False(t, queue.IsEmpty())

	queue.Pop()
	assert.True(t, queue.IsEmpty())
}

// TestLIFOQueue_Push tests adding items to the end of a LIFO queue.
func TestLIFOQueue_Push(t *testing.T) {
	queue := &LIFOQueue[int]{}

	queue.Push(1)
	queue.Push(2)
	queue.Push(3)

	assert.False(t, queue.IsEmpty())

	// Check that items are popped in LIFO order (stack behavior)
	item, ok := queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 3, item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 2, item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 1, item)

	assert.True(t, queue.IsEmpty())
}

// TestLIFOQueue_Pop tests removing items from a LIFO queue in LIFO order.
func TestLIFOQueue_Pop(t *testing.T) {
	queue := &LIFOQueue[string]{}

	// Test empty queue
	_, ok := queue.Pop()
	assert.False(t, ok)

	// Add items
	queue.Push("first")
	queue.Push("second")
	queue.Push("third")

	// Pop and verify LIFO order (stack behavior)
	item, ok := queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, "third", item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, "second", item)

	item, ok = queue.Pop()
	assert.True(t, ok)
	assert.Equal(t, "first", item)

	// Queue should be empty now
	_, ok = queue.Pop()
	assert.False(t, ok)
}

// TestLIFOQueue_IsEmpty tests the IsEmpty method of a LIFO queue.
func TestLIFOQueue_IsEmpty(t *testing.T) {
	queue := &LIFOQueue[int]{}

	assert.True(t, queue.IsEmpty())

	queue.Push(1)
	assert.False(t, queue.IsEmpty())

	queue.Pop()
	assert.True(t, queue.IsEmpty())
}

// TestQueue_Interface tests that both FIFOQueue and LIFOQueue implement the Queue interface.
func TestQueue_Interface(t *testing.T) {
	var fifoQueue Queue[int] = &FIFOQueue[int]{}
	var lifoQueue Queue[int] = &LIFOQueue[int]{}

	// Test that FIFOQueue implements Queue interface
	fifoQueue.Push(1)
	fifoQueue.Push(2)

	item, ok := fifoQueue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 1, item)

	assert.False(t, fifoQueue.IsEmpty())

	// Test that LIFOQueue implements Queue interface
	lifoQueue.Push(1)
	lifoQueue.Push(2)

	item, ok = lifoQueue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 2, item) // LIFO order

	assert.False(t, lifoQueue.IsEmpty())
}

// TestQueue_ComplexTypes tests queue operations with complex types.
func TestQueue_ComplexTypes(t *testing.T) {
	type TestStruct struct {
		ID   int
		Name string
	}

	// Test FIFOQueue with structs
	fifoQueue := &FIFOQueue[TestStruct]{}

	fifoQueue.Push(TestStruct{ID: 1, Name: "first"})
	fifoQueue.Push(TestStruct{ID: 2, Name: "second"})

	item, ok := fifoQueue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 1, item.ID)
	assert.Equal(t, "first", item.Name)

	// Test LIFOQueue with structs
	lifoQueue := &LIFOQueue[TestStruct]{}

	lifoQueue.Push(TestStruct{ID: 1, Name: "first"})
	lifoQueue.Push(TestStruct{ID: 2, Name: "second"})

	item, ok = lifoQueue.Pop()
	assert.True(t, ok)
	assert.Equal(t, 2, item.ID)
	assert.Equal(t, "second", item.Name)
}
