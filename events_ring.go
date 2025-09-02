package main

import "sync/atomic"

const (
	RING_SIZE       = 1 << 16 // 65,536 elements - power of 2 for fast masking
	RING_MASK       = RING_SIZE - 1
	CACHE_LINE_SIZE = 64
)

// Lock-free ring buffer supporting concurrent single producer/consumer
type RingBuffer[T any] struct {
	buffer []T // Fixed-size circular buffer

	// Separate cache lines to avoid false sharing between producer/consumer
	_pad1    [CACHE_LINE_SIZE - 8]byte
	writePos uint64 // Write position
	_pad2    [CACHE_LINE_SIZE - 8]byte
	readPos  uint64 // Read position
	_pad3    [CACHE_LINE_SIZE - 8]byte
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		buffer: make([]T, size),
	}
}

// Push adds one element, busy-waits if buffer is full.
func (r *RingBuffer[T]) Push(v T) {
	for {
		write := atomic.LoadUint64(&r.writePos)
		read := atomic.LoadUint64(&r.readPos)

		if write-read < RING_SIZE { // space available
			r.buffer[write&RING_MASK] = v
			atomic.StoreUint64(&r.writePos, write+1) // publish
			return
		}
		// If ring buffer full → spin
	}
}

// Read extracts up to len(out) elements, busy-waits if buffer is empty.
// Returns the number of elements actually read (≥1).
func (r *RingBuffer[T]) Read(out []T) uint32 {
	for {
		write := atomic.LoadUint64(&r.writePos)
		read := atomic.LoadUint64(&r.readPos)

		available := write - read
		if available == 0 {
			// If ring buffer empty → spin
			continue
		}

		count := min(available, uint64(len(out)))

		for i := uint64(0); i < count; i++ {
			out[i] = r.buffer[(read+i)&RING_MASK]
		}

		atomic.StoreUint64(&r.readPos, read+count)
		return uint32(count)
	}
}

// Exchange engine event types
type EventType uint8

const (
	ORDER_EVENT     EventType = iota // Order creation
	CANCEL_EVENT                     // Order cancellation
	EXECUTION_EVENT                  // Trade execution
	REJECT_EVENT                     // Order rejection
)

// Output event sent by exchange engine
type OutputEvent struct {
	Type           EventType
	OrderID        OrderID
	Price          Price
	Size           Size
	Trader         TraderID
	Symbol         Symbol
	Side           Side
	CounterOrderID OrderID // For executions (counterparty OrderID)
}

// Input command received by exchange engine (related to exchange Order struct)
type InputCommand struct {
	Type    EventType
	Symbol  Symbol
	Side    Side
	Price   Price
	Size    Size
	Trader  TraderID
	OrderID OrderID
}
