package main

const (
	RING_SIZE = 1 << 16 // 65,536 elements - power of 2 for fast masking
	RING_MASK = RING_SIZE - 1
)

// Lock-free ring buffer supporting concurrent single producer/consumer
type RingBuffer[T any] struct {
	buffer   []T    // Fixed-size circular buffer
	writePos uint64 // Atomically updated write position
	readPos  uint64 // Atomically updated read position
}

func NewRingBuffer[T any](size int) *RingBuffer[T] {
	return &RingBuffer[T]{
		buffer: make([]T, size),
	}
}

// Push adds element to buffer (overwrites if full)
func (r *RingBuffer[T]) Push(v T) {
	r.buffer[r.writePos&RING_MASK] = v // Wrap using bit mask
	r.writePos++
}

// Read extracts up to len(out) elements, returns actual count read
func (r *RingBuffer[T]) Read(out []T) uint32 {
	available := r.writePos - r.readPos
	if available == 0 {
		return 0 // Buffer empty
	}

	count := min(available, uint64(len(out)))

	// Copy elements with wraparound handling
	for i := uint64(0); i < count; i++ {
		out[i] = r.buffer[(r.readPos+i)&RING_MASK]
	}

	r.readPos += count
	return uint32(count)
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
