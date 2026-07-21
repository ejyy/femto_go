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

// Order with intrusive linked list for FIFO queues (price/time priority)
type Order struct {
	level    *PriceLevel
	id       OrderID
	gen      Gen  // Generation counter for this order (to avoid stale references)
	prevSlot Slot // Previous order in PriceLevel queue
	nextSlot Slot // Next order in PriceLevel queue
	size     Size
}

type OrderBook struct {
	bidMax Price // Best (highest) bid price
	askMin Price // Best (lowest) ask price

	bidLevels [MAX_PRICE_LEVELS]PriceLevel // Buy order queues by price
	askLevels [MAX_PRICE_LEVELS]PriceLevel // Sell order queues by price
}

func (book *OrderBook) updateBidMax() {
	for price := book.bidMax; price > 0; price-- {
		if book.bidLevels[price].headSlot != 0 {
			book.bidMax = price
			return
		}
	}
	book.bidMax = 0 // No bids remaining
}

func (book *OrderBook) updateAskMin() {
	for price := book.askMin; price < MAX_PRICE_LEVELS; price++ {
		if book.askLevels[price].headSlot != 0 {
			book.askMin = price
			return
		}
	}
	book.askMin = MAX_PRICE_LEVELS // No asks remaining
}

func (book *OrderBook) add(pool *OrderPool, side Side, price Price, id OrderID, slot Slot, size Size) {
	var level *PriceLevel
	if side == Bid {
		level = &book.bidLevels[price]
		if price > book.bidMax {
			book.bidMax = price
		}
	} else {
		level = &book.askLevels[price]
		if price < book.askMin {
			book.askMin = price
		}
	}

	order := pool.get(slot)
	order.id = id
	order.size = size
	level.pushBack(pool, slot)
}

func (book *OrderBook) match(pool *OrderPool, outRing *RingBuffer[OutputEvent], size Size, symbol Symbol, side Side, price Price, trader TraderID, id OrderID) Size {
	remaining := size

	if side == Bid {
		for remaining > 0 && book.askMin < MAX_PRICE_LEVELS && book.askMin <= price {
			remaining = book.matchLevel(&book.askLevels[book.askMin], pool, outRing, remaining, book.askMin, symbol, trader, id)
			if book.askLevels[book.askMin].headSlot == 0 {
				book.updateAskMin()
			}
		}
	} else {
		for remaining > 0 && book.bidMax > 0 && book.bidMax >= price {
			remaining = book.matchLevel(&book.bidLevels[book.bidMax], pool, outRing, remaining, book.bidMax, symbol, trader, id)
			if book.bidLevels[book.bidMax].headSlot == 0 {
				book.updateBidMax()
			}
		}
	}
	return remaining
}

func (book *OrderBook) matchLevel(level *PriceLevel, pool *OrderPool, outRing *RingBuffer[OutputEvent], remaining Size, price Price, symbol Symbol, trader TraderID, id OrderID) Size {
	for counterSlot := level.headSlot; counterSlot != 0 && remaining > 0; {
		counterOrder := pool.get(counterSlot)
		nextCounterSlot := counterOrder.nextSlot

		fillSize := min(remaining, counterOrder.size)

		outRing.Push(OutputEvent{
			eventType:      EXECUTION_EVENT,
			orderID:        id,
			counterOrderID: counterOrder.id,
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
