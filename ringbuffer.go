package main

import "sync/atomic"

const (
	RING_SIZE       = 1 << 16 // 65,536 elements
	RING_MASK       = RING_SIZE - 1
	CACHE_LINE_SIZE = 64
)

// Generic lock-free SPSC ring buffer
type RingBuffer[T any] struct {
	buffer []T
	// Padding to prevent false sharing between producer and consumer
	_ [CACHE_LINE_SIZE - 24]byte

	// Producer-owned cached line
	writePos   uint64 // Current write index (incremented by producer)
	cachedRead uint64 // Cached read index to reduce atomic loads
	_          [CACHE_LINE_SIZE - 16]byte

	// Consumer-owned cached line
	readPos     uint64 // Current read index (incremented by consumer)
	cachedWrite uint64 // Cached write index to reduce atomic loads
	_           [CACHE_LINE_SIZE - 16]byte
}

// Allocates a new, pre-allocated ring buffer instance (of fixed size RING_SIZE)
func NewRingBuffer[T any]() *RingBuffer[T] {
	return &RingBuffer[T]{
		buffer: make([]T, RING_SIZE),
	}
}

// Adds a single element to the ring buffer
func (r *RingBuffer[T]) Push(v T) {
	write := r.writePos

	for {
		// Check if the buffer is full
		if write-r.cachedRead >= RING_SIZE {
			// Refresh cached read position
			r.cachedRead = atomic.LoadUint64(&r.readPos)

			if write-r.cachedRead >= RING_SIZE {
				// Busy-wait until buffer space becomes available
				continue
			}
		}
		break
	}

	// Write the element to the buffer and increment the write position
	r.buffer[write&RING_MASK] = v
	atomic.StoreUint64(&r.writePos, write+1)
}

// Extracts up to len(out) elements from the buffer
func (r *RingBuffer[T]) Read(out []T) uint32 {
	read := r.readPos

	var available uint64

	for {
		if read >= r.cachedWrite {
			// Refresh cached write position
			r.cachedWrite = atomic.LoadUint64(&r.writePos)
		}

		available = r.cachedWrite - read
		if available > 0 {
			break
		}
	}

	count := min(available, uint64(len(out)))

	// Read elements from the buffer and increment the read position
	for i := range count {
		out[i] = r.buffer[(read+i)&RING_MASK]
	}

	atomic.StoreUint64(&r.readPos, read+count)

	return uint32(count)
}
