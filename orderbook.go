package main

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

const (
	Bid Side = iota // Buy orders
	Ask             // Sell orders
)

// Order with intrusive linked FIFO list within its price level (price/time priority)
type Order struct {
	price    Price
	size     Size
	gen      Gen  // Generation counter (to avoid stale references)
	prevSlot Slot // Previous order in price level queue
	nextSlot Slot // Next order in price level queue
	symbol   Symbol
	side     Side
}

type OrderBook struct {
	bidMax Price // Best (highest) bid price
	askMin Price // Best (lowest) ask price

	bidLevels [MAX_PRICE_LEVELS]PriceLevel // Buy order queues by price
	askLevels [MAX_PRICE_LEVELS]PriceLevel // Sell order queues by price
}

// Finds next highest price containing bids after current level becomes empty
func (book *OrderBook) updateBidMax() {
	for price := book.bidMax; price > 0; price-- {
		if book.bidLevels[price].headSlot != 0 {
			book.bidMax = price
			return
		}
	}
	book.bidMax = 0 // No bids remaining
}

// Finds next lowest price containing asks after current level becomes empty
func (book *OrderBook) updateAskMin() {
	for price := book.askMin; price < MAX_PRICE_LEVELS; price++ {
		if book.askLevels[price].headSlot != 0 {
			book.askMin = price
			return
		}
	}
	book.askMin = MAX_PRICE_LEVELS // No asks remaining
}

// Returns the price level queue for the given side and price
func (book *OrderBook) level(side Side, price Price) *PriceLevel {
	if side == Bid {
		return &book.bidLevels[price]
	}
	return &book.askLevels[price]
}

// Inserts an unfilled order at the back of its price level queue
func (book *OrderBook) add(pool *OrderPool, side Side, price Price, slot Slot, size Size, symbol Symbol) {
	level := book.level(side, price)

	// Update best bid/ask prices if order improves it
	if side == Bid {
		if price > book.bidMax {
			book.bidMax = price
		}
	} else {
		if price < book.askMin {
			book.askMin = price
		}
	}

	order := pool.get(slot)
	order.size = size
	order.side = side
	order.price = price
	order.symbol = symbol

	level.pushBack(pool, slot)
}

// Consumes incoming order against eligible orders in the book, starting at best available price
func (book *OrderBook) match(pool *OrderPool, outRing *RingBuffer[OutputEvent], size Size, symbol Symbol, side Side, price Price, trader TraderID, id OrderID) Size {
	remaining := size

	if side == Bid {
		// A bid can match asks priced at or below the bid price
		for remaining > 0 && book.askMin < MAX_PRICE_LEVELS && book.askMin <= price {
			remaining = book.matchLevel(&book.askLevels[book.askMin], pool, outRing, remaining, book.askMin, symbol, trader, id)
			if book.askLevels[book.askMin].headSlot == 0 {
				book.updateAskMin()
			}
		}
	} else {
		// An ask can match bids priced at or above the ask price
		for remaining > 0 && book.bidMax > 0 && book.bidMax >= price {
			remaining = book.matchLevel(&book.bidLevels[book.bidMax], pool, outRing, remaining, book.bidMax, symbol, trader, id)
			if book.bidLevels[book.bidMax].headSlot == 0 {
				book.updateBidMax()
			}
		}
	}
	return remaining
}

// Fills orders at one price level in FIFO order until the incoming order is fully matched or the level is exhausted
func (book *OrderBook) matchLevel(level *PriceLevel, pool *OrderPool, outRing *RingBuffer[OutputEvent], remaining Size, price Price, symbol Symbol, trader TraderID, id OrderID) Size {
	for counterSlot := level.headSlot; counterSlot != 0 && remaining > 0; {
		counterOrder := pool.get(counterSlot)
		nextCounterSlot := counterOrder.nextSlot // Save before removal

		fillSize := min(remaining, counterOrder.size)

		outRing.Push(OutputEvent{
			eventType:      EXECUTION_EVENT,
			orderID:        id,
			counterOrderID: OrderID(uint64(counterOrder.gen)<<SLOT_BITS | uint64(counterSlot)),
			price:          price,
			size:           fillSize,
			trader:         trader,
			symbol:         symbol,
		})

		remaining -= fillSize
		counterOrder.size -= fillSize

		if counterOrder.size == 0 {
			level.remove(pool, counterSlot)
		}
		counterSlot = nextCounterSlot
	}
	return remaining
}
