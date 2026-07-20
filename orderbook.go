package main

// Constants for matching engine
const (
	MAX_PRICE_LEVELS = 1 << 14 // 16,384 price ticks

	SLOT_BITS = 26
	SLOT_MASK = (1 << SLOT_BITS) - 1
)

// Type definitions for Order constituents
type (
	OrderID  uint64
	Price    uint32
	Size     uint32
	TraderID uint16
	Symbol   uint16
	Side     uint8
	Slot     uint32
	Gen      uint32
)

// Side types
const (
	Bid Side = iota // Buy orders
	Ask             // Sell orders
)

// Order with intrusive linked list for FIFO queues (price/time priority)
type Order struct {
	level    *PriceLevel
	id       OrderID
	gen      Gen  // Generation counter for this order (to avoid stale references)
	prevSlot Slot // Previous order in PriceLevel queue
	nextSlot Slot // Next order in PriceLevel queue
	size     Size
}

// Orderbook with separate bid/ask price levels
type OrderBook struct {
	bidMax Price // Best (highest) bid price
	askMin Price // Best (lowest) ask price

	bidLevels [MAX_PRICE_LEVELS]PriceLevel // Buy order queues by price
	askLevels [MAX_PRICE_LEVELS]PriceLevel // Sell order queues by price
}

// Pricelevel serving as a FIFO queue of orders at a specific price
type PriceLevel struct {
	headSlot Slot   // First order (oldest)
	tailSlot Slot   // Last order (newest)
	count    uint32 // Total number of discrete orders at this level (not volume)
}

// updateBestBid scans for the next best bid price (descending)
func (book *OrderBook) updateBidMax() {
	for price := book.bidMax; price > 0; price-- {
		if book.bidLevels[price].count > 0 {
			book.bidMax = price
			return
		}
	}
	book.bidMax = 0 // No bids remaining
}

// updateBestAsk scans for the next best ask price (ascending)
func (book *OrderBook) updateAskMin() {
	for price := book.askMin; price < MAX_PRICE_LEVELS; price++ {
		if book.askLevels[price].count > 0 {
			book.askMin = price
			return
		}
	}
	book.askMin = MAX_PRICE_LEVELS // No asks remaining
}

func makeOrderID(slot Slot, gen Gen) OrderID {
	return OrderID(uint64(gen)<<SLOT_BITS | uint64(slot))
}
func (id OrderID) slot() Slot { return Slot(id & SLOT_MASK) }
func (id OrderID) gen() Gen   { return Gen(id >> SLOT_BITS) }
