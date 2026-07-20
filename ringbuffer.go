package main

import "sync/atomic"

// Constants defining the ring buffer properties
const (
	RING_SIZE       = 1 << 16       // 65,536 elements - must be a power of 2 for efficient masking
	RING_MASK       = RING_SIZE - 1 // Mask for fast modulo operation using bitwise AND
	CACHE_LINE_SIZE = 64            // Typical CPU cache line size to avoid false sharing
)

// Lock-free ring buffer supporting a single producer and a single consumer (SPSC)
// Generic type T allows storing any type of element.
type RingBuffer[T any] struct {
	buffer []T // Fixed-size circular buffer to hold elements

	// Padding arrays to ensure writePos and readPos are on separate cache lines.
	// This prevents "false sharing," where different cores repeatedly write to
	// memory that shares the same cache line, causing performance degradation.
	_pad1    [CACHE_LINE_SIZE - 8]byte // padding before writePos
	writePos uint64                    // Current write index (incremented by producer)
	_pad2    [CACHE_LINE_SIZE - 8]byte // padding before readPos
	readPos  uint64                    // Current read index (incremented by consumer)
	_pad3    [CACHE_LINE_SIZE - 8]byte // padding after readPos
}

// NewRingBuffer allocates and returns a pointer to a new ring buffer instance.
// Initialises the internal buffer with a fixed size (RING_SIZE elements).
func NewRingBuffer[T any]() *RingBuffer[T] {
	return &RingBuffer[T]{
		buffer: make([]T, RING_SIZE), // preallocate memory for ring buffer
	}
}

// Push adds a single element to the ring buffer.
// This is a busy-waiting (spin) implementation if the buffer is full.
// Only safe for a single producer; concurrent Push calls would be unsafe.
func (r *RingBuffer[T]) Push(v T) {
	for {
		// Atomically load the current write and read positions
		write := atomic.LoadUint64(&r.writePos)
		read := atomic.LoadUint64(&r.readPos)

		// Calculate available space by checking difference between write and read indices
		if write-read < RING_SIZE { // There is space in the buffer
			// Compute actual index using bitwise AND with mask (fast modulo)
			r.buffer[write&RING_MASK] = v
			// Publish the new write position atomically
			atomic.StoreUint64(&r.writePos, write+1)
			return
		}

		// If buffer is full, loop (busy-wait) until space becomes available
	}
}

// Read extracts up to len(out) elements from the buffer.
// Returns the number of elements actually read (always ≥ 1).
// This is a busy-waiting (spin) implementation if the buffer is empty.
// Only safe for a single consumer; concurrent Read calls would be unsafe.
func (r *RingBuffer[T]) Read(out []T) uint32 {
	for {
		// Atomically load the current write and read positions
		write := atomic.LoadUint64(&r.writePos)
		read := atomic.LoadUint64(&r.readPos)

		// Calculate how many elements are available to read
		available := write - read
		if available == 0 {
			// If buffer is empty, loop (busy-wait) until elements are written
			continue
		}

		// Determine how many elements we can actually read
		count := min(available, uint64(len(out)))

		// Copy elements from buffer into output slice
		for i := uint64(0); i < count; i++ {
			// Use bitwise AND with mask to wrap around the circular buffer
			out[i] = r.buffer[(read+i)&RING_MASK]
		}

		// Update read position to mark elements as consumed
		atomic.StoreUint64(&r.readPos, read+count)
		return uint32(count) // Return the number of elements read
	}
}
