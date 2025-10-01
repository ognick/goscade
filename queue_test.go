package goscade

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test: FIFOQueue Enqueue adds items to the end
func TestFIFOQueue_Enqueue(t *testing.T) {
	queue := &FIFOQueue[int]{}

	queue.Enqueue(1)
	queue.Enqueue(2)
	queue.Enqueue(3)

	assert.False(t, queue.IsEmpty())

	// Check that items are dequeued in FIFO order
	item, ok := queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 1, item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 2, item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 3, item)

	assert.True(t, queue.IsEmpty())
}

// Test: FIFOQueue Dequeue returns items in correct order
func TestFIFOQueue_Dequeue(t *testing.T) {
	queue := &FIFOQueue[string]{}

	// Test empty queue
	_, ok := queue.Dequeue()
	assert.False(t, ok)

	// Add items
	queue.Enqueue("first")
	queue.Enqueue("second")
	queue.Enqueue("third")

	// Dequeue and verify order
	item, ok := queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "first", item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "second", item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "third", item)

	// Queue should be empty now
	_, ok = queue.Dequeue()
	assert.False(t, ok)
}

// Test: FIFOQueue IsEmpty returns correct state
func TestFIFOQueue_IsEmpty(t *testing.T) {
	queue := &FIFOQueue[int]{}

	assert.True(t, queue.IsEmpty())

	queue.Enqueue(1)
	assert.False(t, queue.IsEmpty())

	queue.Dequeue()
	assert.True(t, queue.IsEmpty())
}

// Test: LIFOQueue Enqueue adds items to the end
func TestLIFOQueue_Enqueue(t *testing.T) {
	queue := &LIFOQueue[int]{}

	queue.Enqueue(1)
	queue.Enqueue(2)
	queue.Enqueue(3)

	assert.False(t, queue.IsEmpty())

	// Check that items are dequeued in LIFO order (stack behavior)
	item, ok := queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 3, item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 2, item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 1, item)

	assert.True(t, queue.IsEmpty())
}

// Test: LIFOQueue Dequeue returns items in LIFO order
func TestLIFOQueue_Dequeue(t *testing.T) {
	queue := &LIFOQueue[string]{}

	// Test empty queue
	_, ok := queue.Dequeue()
	assert.False(t, ok)

	// Add items
	queue.Enqueue("first")
	queue.Enqueue("second")
	queue.Enqueue("third")

	// Dequeue and verify LIFO order (stack behavior)
	item, ok := queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "third", item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "second", item)

	item, ok = queue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, "first", item)

	// Queue should be empty now
	_, ok = queue.Dequeue()
	assert.False(t, ok)
}

// Test: LIFOQueue IsEmpty returns correct state
func TestLIFOQueue_IsEmpty(t *testing.T) {
	queue := &LIFOQueue[int]{}

	assert.True(t, queue.IsEmpty())

	queue.Enqueue(1)
	assert.False(t, queue.IsEmpty())

	queue.Dequeue()
	assert.True(t, queue.IsEmpty())
}

// Test: Queue interface implementation
func TestQueue_Interface(t *testing.T) {
	var fifoQueue Queue[int] = &FIFOQueue[int]{}
	var lifoQueue Queue[int] = &LIFOQueue[int]{}

	// Test FIFOQueue implements Queue interface
	fifoQueue.Enqueue(1)
	fifoQueue.Enqueue(2)

	item, ok := fifoQueue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 1, item)

	assert.False(t, fifoQueue.IsEmpty())

	// Test LIFOQueue implements Queue interface
	lifoQueue.Enqueue(1)
	lifoQueue.Enqueue(2)

	item, ok = lifoQueue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 2, item) // LIFO order

	assert.False(t, lifoQueue.IsEmpty())
}

// Test: Queue with complex types
func TestQueue_ComplexTypes(t *testing.T) {
	type TestStruct struct {
		ID   int
		Name string
	}

	// Test FIFOQueue with structs
	fifoQueue := &FIFOQueue[TestStruct]{}

	fifoQueue.Enqueue(TestStruct{ID: 1, Name: "first"})
	fifoQueue.Enqueue(TestStruct{ID: 2, Name: "second"})

	item, ok := fifoQueue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 1, item.ID)
	assert.Equal(t, "first", item.Name)

	// Test LIFOQueue with structs
	lifoQueue := &LIFOQueue[TestStruct]{}

	lifoQueue.Enqueue(TestStruct{ID: 1, Name: "first"})
	lifoQueue.Enqueue(TestStruct{ID: 2, Name: "second"})

	item, ok = lifoQueue.Dequeue()
	assert.True(t, ok)
	assert.Equal(t, 2, item.ID)
	assert.Equal(t, "second", item.Name)
}
