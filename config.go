package main

// Constants for order book and matching engine
const (
	MAX_SYMBOLS      = 1 << 8  // 256 trading symbols
	MAX_PRICE_LEVELS = 1 << 14 // 16,384 price ticks

	SLOT_BITS = 26
	SLOT_MASK = (1 << SLOT_BITS) - 1

	MAX_ORDERS = 1 << SLOT_BITS // 67M total orders
)

// Constant defining the message bus buffer size
const (
	DISTRIBUTOR_BUFFER = 1 << 10 // 1024 events size
)

// Constants defining the ring buffer properties
const (
	RING_SIZE       = 1 << 16       // 65,536 elements - must be a power of 2 for efficient masking
	RING_MASK       = RING_SIZE - 1 // Mask for fast modulo operation using bitwise AND
	CACHE_LINE_SIZE = 64            // Typical CPU cache line size to avoid false sharing
)
