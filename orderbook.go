package main

// Constants for matching engine
const (
	MAX_PRICE_LEVELS = 1 << 14 // 16,384 price ticks
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

// Insert order into appropriate price level queue (FIFO)
func (e *MatchingEngine) addToBook(book *OrderBook, size Size, oSide Side, oPrice Price, oID OrderID, slot Slot) {
	var level *PriceLevel
	if oSide == Bid {
		level = &book.bidLevels[oPrice]
		if oPrice > book.bidMax {
			book.bidMax = oPrice
		}
	} else {
		level = &book.askLevels[oPrice]
		if oPrice < book.askMin {
			book.askMin = oPrice
		}
	}

	order := &e.orders[slot]
	order.level = level
	order.id = oID
	order.size = size
	order.prevSlot, order.nextSlot = 0, 0

	if level.headSlot == 0 {
		level.headSlot = slot
	} else {
		tail := &e.orders[level.tailSlot]
		tail.nextSlot = slot
		order.prevSlot = level.tailSlot
	}
	level.tailSlot = slot
	level.count++
}

// Remove order from doubly-linked list maintaining FIFO integrity
func (e *MatchingEngine) unlink(level *PriceLevel, slot Slot) {
	order := &e.orders[slot]
	if order.prevSlot != 0 {
		e.orders[order.prevSlot].nextSlot = order.nextSlot
	} else {
		level.headSlot = order.nextSlot
	}
	if order.nextSlot != 0 {
		e.orders[order.nextSlot].prevSlot = order.prevSlot
	} else {
		level.tailSlot = order.prevSlot
	}
	level.count--
	e.freeSlot(slot)
}
