package main

// Exchange engine with pre-allocated arrays
type MatchingEngine struct {
	books [MAX_SYMBOLS]OrderBook // Order books per symbol

	orders [MAX_ORDERS]Order // Pre-allocated order pool

	orderID OrderID // Monotonic order ID generator

	freeHead     Slot
	nextFreeSlot Slot

	inputRing  *RingBuffer[InputCommand] // Incoming commands
	outputRing *RingBuffer[OutputEvent]  // Outgoing events
}

func NewMatchingEngine() *MatchingEngine {
	e := &MatchingEngine{
		inputRing:  NewRingBuffer[InputCommand](),
		outputRing: NewRingBuffer[OutputEvent](),
	}

	// Set ask minimum to initial value (no asks)
	for i := range e.books {
		e.books[i] = OrderBook{
			askMin: MAX_PRICE_LEVELS,
			bidMax: 0,
		}
	}

	return e
}

// Process limit order with matching and book insertion
func (e *MatchingEngine) Limit(symbol Symbol, side Side, price Price, size Size, trader TraderID) {
	if price == 0 || size == 0 || price >= MAX_PRICE_LEVELS {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	slot, gen := e.allocSlot()
	newOrderID := makeOrderID(slot, gen)

	e.outputRing.Push(OutputEvent{
		eventType: ORDER_EVENT,
		orderID:   newOrderID,
		price:     price, size: size, trader: trader, symbol: symbol, side: side,
	})

	book := &e.books[symbol]
	remaining := e.match(book, size, symbol, side, price, trader, newOrderID)

	if remaining > 0 {
		e.addToBook(book, remaining, side, price, newOrderID, slot)
	} else {
		e.freeSlot(slot) // fully filled, never entered book — recycle immediately
	}
}

func (e *MatchingEngine) allocSlot() (Slot, uint32) {
	var slot Slot
	if e.freeHead != 0 {
		slot = e.freeHead
		e.freeHead = e.orders[slot].nextSlot
	} else {
		e.nextFreeSlot++
		slot = e.nextFreeSlot
	}
	return slot, e.orders[slot].gen
}

func (e *MatchingEngine) freeSlot(slot Slot) {
	order := &e.orders[slot]
	order.gen++
	order.nextSlot = e.freeHead
	e.freeHead = slot
}

// Match incoming order against opposite side of book
func (e *MatchingEngine) match(book *OrderBook, oSize Size, oSymbol Symbol, oSide Side, oPrice Price, oTrader TraderID, oID OrderID) (remaining Size) {
	remaining = oSize

	if oSide == Bid {
		// Buy order matches against asks at or below bid price
		for remaining > 0 && book.askMin < MAX_PRICE_LEVELS && book.askMin <= oPrice {
			remaining = e.matchLevel(&book.askLevels[book.askMin], remaining, book.askMin, oSymbol, oTrader, oID)
			if remaining > 0 && book.askLevels[book.askMin].headSlot == 0 { // Only checks if PriceLevel exhausted
				book.updateAskMin() // Find next best ask
			}
		}
	} else {
		// Sell order matches against bids at or above ask price
		for remaining > 0 && book.bidMax > 0 && book.bidMax >= oPrice {
			remaining = e.matchLevel(&book.bidLevels[book.bidMax], remaining, book.bidMax, oSymbol, oTrader, oID)
			if remaining > 0 && book.bidLevels[book.bidMax].headSlot == 0 { // Only checks if PriceLevel exhausted
				book.updateBidMax() // Find next best bid
			}
		}
	}

	return remaining
}

// Execute trades against orders at specific price level (FIFO)
func (e *MatchingEngine) matchLevel(level *PriceLevel, remaining Size, price Price, oSymbol Symbol, oTrader TraderID, oID OrderID) Size {
	for counterSlot := level.headSlot; counterSlot != 0 && remaining > 0; {
		counterOrder := &e.orders[counterSlot]
		nextCounterSlot := counterOrder.nextSlot // Save before potential unlink

		fillSize := min(remaining, counterOrder.size)

		// Report trade execution
		e.outputRing.Push(OutputEvent{
			eventType:      EXECUTION_EVENT,
			orderID:        oID,
			price:          price, // Trade at resting order price
			size:           fillSize,
			trader:         oTrader,
			symbol:         oSymbol,
			counterOrderID: counterOrder.id,
		})

		remaining -= fillSize
		counterOrder.size -= fillSize

		// Remove fully filled orders
		if counterOrder.size == 0 {
			e.unlink(level, counterSlot)
		}

		counterSlot = nextCounterSlot
	}

	return remaining
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

// Cancel order by removing from price level queue
func (e *MatchingEngine) Cancel(id OrderID) {
	slot := id.slot()
	if slot == 0 || slot > e.nextFreeSlot {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}
	order := &e.orders[slot]
	if order.gen != id.gen() || order.size == 0 {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT}) // stale/reused/already-cancelled ID
		return
	}
	e.unlink(order.level, slot)
	order.size = 0
	e.outputRing.Push(OutputEvent{eventType: CANCEL_EVENT, orderID: id})
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
