package main

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

func (book *OrderBook) add(pool *OrderPool, oSide Side, oPrice Price, oID OrderID, slot Slot, size Size) {
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

	order := pool.get(slot)
	order.id = oID
	order.size = size
	level.pushBack(pool, slot)
}

func (book *OrderBook) match(pool *OrderPool, outRing *RingBuffer[OutputEvent], oSize Size, oSymbol Symbol, oSide Side, oPrice Price, oTrader TraderID, oID OrderID) Size {
	remaining := oSize

	if oSide == Bid {
		for remaining > 0 && book.askMin < MAX_PRICE_LEVELS && book.askMin <= oPrice {
			remaining = book.matchLevel(&book.askLevels[book.askMin], pool, outRing, remaining, book.askMin, oSymbol, oTrader, oID)
			if remaining > 0 && book.askLevels[book.askMin].headSlot == 0 {
				book.updateAskMin()
			}
		}
	} else {
		for remaining > 0 && book.bidMax > 0 && book.bidMax >= oPrice {
			remaining = book.matchLevel(&book.bidLevels[book.bidMax], pool, outRing, remaining, book.bidMax, oSymbol, oTrader, oID)
			if remaining > 0 && book.bidLevels[book.bidMax].headSlot == 0 {
				book.updateBidMax()
			}
		}
	}
	return remaining
}

func (book *OrderBook) matchLevel(level *PriceLevel, pool *OrderPool, outRing *RingBuffer[OutputEvent], remaining Size, price Price, oSymbol Symbol, oTrader TraderID, oID OrderID) Size {
	for counterSlot := level.headSlot; counterSlot != 0 && remaining > 0; {
		counterOrder := pool.get(counterSlot)
		nextCounterSlot := counterOrder.nextSlot

		fillSize := min(remaining, counterOrder.size)

		// Direct push restored
		outRing.Push(OutputEvent{
			eventType:      EXECUTION_EVENT,
			orderID:        oID,
			counterOrderID: counterOrder.id,
			price:          price,
			size:           fillSize,
			trader:         oTrader,
			symbol:         oSymbol,
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
