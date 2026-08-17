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
	// Procuder-owned cached line
	_pad1      [CACHE_LINE_SIZE - 16]byte // padding before writePos
	writePos   uint64                     // Current write index (incremented by producer)
	cachedRead uint64                     // Cached read index to reduce atomic loads

	// Consumer-owned cached line
	_pad2       [CACHE_LINE_SIZE - 16]byte // padding before readPos
	readPos     uint64                     // Current read index (incremented by consumer)
	cachedWrite uint64                     // Cached write index to reduce atomic loads

	_pad3 [CACHE_LINE_SIZE]byte // padding after readPos
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
	write := r.writePos

	for {
		if write-r.cachedRead >= RING_SIZE {
			r.cachedRead = atomic.LoadUint64(&r.readPos) // Refresh cached read position

			if write-r.cachedRead >= RING_SIZE {
				// Buffer is full; busy-wait until space becomes available
				continue
			}
		}
		break
	}

	r.buffer[write&RING_MASK] = v
	atomic.StoreUint64(&r.writePos, write+1)
}

// Read extracts up to len(out) elements from the buffer.
// Returns the number of elements actually read (always ≥ 1).
// This is a busy-waiting (spin) implementation if the buffer is empty.
// Only safe for a single consumer; concurrent Read calls would be unsafe.
func (r *RingBuffer[T]) Read(out []T) uint32 {
	read := r.readPos

	var available uint64

	for {
		if read >= r.cachedWrite {
			r.cachedWrite = atomic.LoadUint64(&r.writePos) // Refresh cached write position
		}

		available = r.cachedWrite - read
		if available > 0 {
			break
		}
	}

	count := min(available, uint64(len(out)))

	for i := range count {
		out[i] = r.buffer[(read+i)&RING_MASK]
	}

	atomic.StoreUint64(&r.readPos, read+count)

	return uint32(count)
}
