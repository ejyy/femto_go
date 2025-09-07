package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestNewRingBufferInitialization ensures that a new ring buffer is
// properly initialised with the correct size and initial positions.
func TestNewRingBufferInitialization(t *testing.T) {
	rb := NewRingBuffer[int]()

	if rb == nil {
		t.Fatal("RingBuffer should not be nil after initialization")
	}
	if len(rb.buffer) != RING_SIZE {
		t.Fatalf("Expected buffer size %d, got %d", RING_SIZE, len(rb.buffer))
	}
	if rb.writePos != 0 || rb.readPos != 0 {
		t.Fatalf("Expected initial writePos and readPos to be 0, got %d and %d", rb.writePos, rb.readPos)
	}
}

// TestPushAndReadSingleElement checks that a single value can be pushed
// and read correctly from the buffer.
func TestPushAndReadSingleElement(t *testing.T) {
	rb := NewRingBuffer[int]()

	rb.Push(42)           // Push a single element
	out := make([]int, 1) // Allocate slice to read into
	n := rb.Read(out)

	if n != 1 {
		t.Fatalf("Expected to read 1 element, got %d", n)
	}
	if out[0] != 42 {
		t.Fatalf("Expected value 42, got %d", out[0])
	}
}

// TestPushAndReadMultipleElements ensures multiple sequential pushes
// and reads preserve order and correctness.
func TestPushAndReadMultipleElements(t *testing.T) {
	rb := NewRingBuffer[int]()
	values := []int{1, 2, 3, 4, 5}

	// Push multiple elements
	for _, v := range values {
		rb.Push(v)
	}

	out := make([]int, len(values))
	n := rb.Read(out)

	if int(n) != len(values) {
		t.Fatalf("Expected to read %d elements, got %d", len(values), n)
	}

	// Verify order is preserved
	for i, v := range values {
		if out[i] != v {
			t.Errorf("Expected %d at index %d, got %d", v, i, out[i])
		}
	}
}

// TestRingBufferWrapAround tests proper handling of the circular buffer
// when indices wrap past the end of the internal array.
func TestRingBufferWrapAround(t *testing.T) {
	rb := NewRingBuffer[int]() // Fixed-size buffer

	// Step 1: Fill the buffer completely
	for i := 0; i < RING_SIZE; i++ {
		rb.Push(i)
	}

	// Step 2: Read half of the buffer
	out := make([]int, RING_SIZE/2)
	n := rb.Read(out)
	if int(n) != RING_SIZE/2 {
		t.Fatalf("Expected to read %d items, got %d", RING_SIZE/2, n)
	}

	// Step 3: Push another half set of values to force wrap-around
	for i := 0; i < RING_SIZE/2; i++ {
		rb.Push(1000000 + i)
	}

	// Step 4: Read the remaining old values
	oldValues := make([]int, RING_SIZE/2)
	n = rb.Read(oldValues)
	if int(n) != RING_SIZE/2 {
		t.Fatalf("Expected to read %d old values, got %d", RING_SIZE/2, n)
	}
	for i, v := range oldValues {
		expected := RING_SIZE/2 + i
		if v != expected {
			t.Fatalf("Old value mismatch at index %d: expected %d, got %d", i, expected, v)
		}
	}

	// Step 5: Read the new wrapped-around values
	newValues := make([]int, RING_SIZE/2)
	n = rb.Read(newValues)
	if int(n) != RING_SIZE/2 {
		t.Fatalf("Expected to read %d wrapped values, got %d", RING_SIZE/2, n)
	}
	for i, v := range newValues {
		expected := 1000000 + i
		if v != expected {
			t.Fatalf("Wrap-around data mismatch at index %d: expected %d, got %d", i, expected, v)
		}
	}
}

// TestConcurrentProducerConsumer tests the ring buffer under concurrent
// producer and consumer operations.
func TestConcurrentProducerConsumer(t *testing.T) {
	rb := NewRingBuffer[int]()
	const total = 100000
	var wg sync.WaitGroup

	// Producer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < total; i++ {
			rb.Push(i)
		}
	}()

	// Consumer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		out := make([]int, 256) // Read in batches
		readCount := 0
		for readCount < total {
			n := rb.Read(out)
			readCount += int(n)
		}
		if readCount != total {
			t.Errorf("Expected to read %d elements, got %d", total, readCount)
		}
	}()

	wg.Wait() // Wait for both producer and consumer
}

// TestEmptyBufferReadBlocksUntilPush ensures that a Read blocks if the buffer
// is empty until a Push occurs.
func TestEmptyBufferReadBlocksUntilPush(t *testing.T) {
	rb := NewRingBuffer[int]()
	out := make([]int, 1)
	done := make(chan struct{})

	go func() {
		n := rb.Read(out) // This should block until a value is pushed
		if n != 1 {
			t.Errorf("Expected to read 1 element, got %d", n)
		}
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // Give goroutine time to start and block
	rb.Push(99)                       // Unblock the reader

	select {
	case <-done:
		// Read successfully unblocked
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Read did not unblock after Push")
	}

	if out[0] != 99 {
		t.Fatalf("Expected value 99, got %d", out[0])
	}
}

// TestFullBufferPushBlocksUntilRead ensures that a Push blocks if the buffer
// is full until a Read frees space.
func TestFullBufferPushBlocksUntilRead(t *testing.T) {
	rb := NewRingBuffer[int]()

	// Fill the buffer completely
	for i := 0; i < RING_SIZE; i++ {
		rb.Push(i)
	}

	started := time.Now()
	done := make(chan struct{})

	// Attempt to push into full buffer
	go func() {
		rb.Push(12345) // This should block until a slot is freed
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // Ensure goroutine is blocked

	// Free one slot
	out := make([]int, 1)
	rb.Read(out)

	select {
	case <-done:
		// Push successfully unblocked
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Push did not unblock after Read")
	}

	// Verify the buffer size invariant: writePos - readPos should equal RING_SIZE
	if atomic.LoadUint64(&rb.writePos)-atomic.LoadUint64(&rb.readPos) != RING_SIZE {
		t.Fatalf("Buffer size invariant broken")
	}

	if time.Since(started) < 50*time.Millisecond {
		t.Fatal("Push did not block as expected when buffer was full")
	}
}

// TestGenericSupport ensures that the ring buffer works with custom types.
func TestGenericSupport(t *testing.T) {
	type custom struct {
		ID   int
		Name string
	}
	rb := NewRingBuffer[custom]()

	val := custom{ID: 1, Name: "test"}
	rb.Push(val)

	out := make([]custom, 1)
	n := rb.Read(out)

	if n != 1 {
		t.Fatalf("Expected to read 1 element, got %d", n)
	}
	if out[0] != val {
		t.Fatalf("Expected %+v, got %+v", val, out[0])
	}
}
