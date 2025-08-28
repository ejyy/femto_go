package main

type OrderID uint32
type Price uint32
type Size uint32
type TraderID uint16
type Side uint8
type Symbol uint16

const (
	Bid Side = iota // Buy orders
	Ask             // Sell orders
)

// Order with intrusive linked list for FIFO queues (price/time priority)
type Order struct {
	Prev  OrderID // Previous order in PriceLevel queue
	Next  OrderID // Next order in PriceLevel queue
	Level *PriceLevel
	Size  Size
}

// Order book with separate bid/ask price levels
type OrderBook struct {
	bidMax Price // Best (highest) bid price
	askMin Price // Best (lowest) ask price

	bidLevels [MAX_PRICE_LEVELS]PriceLevel // Buy order queues by price
	askLevels [MAX_PRICE_LEVELS]PriceLevel // Sell order queues by price
}

// FIFO queue of orders at a specific price level
type PriceLevel struct {
	head OrderID // First order (oldest)
	tail OrderID // Last order (newest)
	size uint32  // Total orders at this level
}

// Scan for next best bid price (descending)
func (book *OrderBook) updateBestBid() {
	for price := book.bidMax; price > 0; price-- {
		if book.bidLevels[price].size > 0 {
			book.bidMax = price
			return
		}
	}
	book.bidMax = 0 // No bids remaining
}

// Scan for next best ask price (ascending)
func (book *OrderBook) updateBestAsk() {
	for price := book.askMin; price < MAX_PRICE_LEVELS; price++ {
		if book.askLevels[price].size > 0 {
			book.askMin = price
			return
		}
	}
	book.askMin = MAX_PRICE_LEVELS // No asks remaining
}
