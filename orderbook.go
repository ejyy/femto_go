package main

// Type definitions for Order constituents
type (
	OrderID  uint32
	Price    uint32
	Size     uint32
	TraderID uint16
	Symbol   uint16
	Side     uint8
)

// Side types
const (
	Bid Side = iota // Buy orders
	Ask             // Sell orders
)

// Order with intrusive linked list for FIFO queues (price/time priority)
type Order struct {
	level *PriceLevel
	prev  OrderID // Previous order in PriceLevel queue
	next  OrderID // Next order in PriceLevel queue
	size  Size
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
	head OrderID // First order (oldest)
	tail OrderID // Last order (newest)
	size uint32  // Total number of discrete orders at this level (not volume)
}

// updateBestBid scans for the next best bid price (descending)
func (book *OrderBook) updateBestBid() {
	for price := book.bidMax; price > 0; price-- {
		if book.bidLevels[price].size > 0 {
			book.bidMax = price
			return
		}
	}
	book.bidMax = 0 // No bids remaining
}

// updateBestAsk scans for the next best ask price (ascending)
func (book *OrderBook) updateBestAsk() {
	for price := book.askMin; price < MAX_PRICE_LEVELS; price++ {
		if book.askLevels[price].size > 0 {
			book.askMin = price
			return
		}
	}
	book.askMin = MAX_PRICE_LEVELS // No asks remaining
}
