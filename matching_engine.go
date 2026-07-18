package main

const (
	MAX_SYMBOLS      = 1 << 8         // 256 trading symbols
	MAX_PRICE_LEVELS = 1 << 14        // 16,384 price ticks
	MAX_ORDERS       = 1 << 26        // 67M total orders
	FREE_SLOTS       = 1 << 10        // 1024 free order slots
	FREE_MASK        = FREE_SLOTS - 1 // Free slot mask
)

// Exchange engine with pre-allocated arrays
type MatchingEngine struct {
	books [MAX_SYMBOLS]OrderBook // Order books per symbol

	orders     [MAX_ORDERS]Order // Pre-allocated order pool
	orderIndex [MAX_ORDERS]Slot  // External OrderID -> internal slot index

	orderID OrderID // Monotonic order ID generator

	freeSlots [FREE_SLOTS]uint32 // 'Recycled' Order slots
	freeHead  uint32             // First free slot
	freeTail  uint32             // Next empty slot

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
	// Validate order parameters
	if price == 0 || size == 0 || price >= MAX_PRICE_LEVELS {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	e.orderID++
	newOrderID := e.orderID

	// Report order receipt
	e.outputRing.Push(OutputEvent{
		eventType: ORDER_EVENT,
		orderID:   newOrderID,
		price:     price,
		size:      size,
		trader:    trader,
		symbol:    symbol,
		side:      side,
	})

	// Lookup and match according to symbol
	book := &e.books[symbol]
	remaining := e.match(book, size, symbol, side, price, trader, newOrderID)

	// Add unfilled portion to book
	if remaining > 0 {
		e.addToBook(book, remaining, side, price, newOrderID)
	}
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
func (e *MatchingEngine) addToBook(book *OrderBook, size Size, oSide Side, oPrice Price, oID OrderID) {
	var level *PriceLevel

	if oSide == Bid {
		level = &book.bidLevels[oPrice]
		// Update best bid if this is higher
		if oPrice > book.bidMax {
			book.bidMax = oPrice
		}
	} else {
		level = &book.askLevels[oPrice]
		// Update best ask if this is lower
		if oPrice < book.askMin {
			book.askMin = oPrice
		}
	}

	// Pick internal slot (recycled or new)
	var slot Slot
	if e.freeHead != e.freeTail {
		slot = Slot(e.freeSlots[e.freeHead&FREE_MASK])
		e.freeHead++
	} else {
		slot = Slot(oID)
	}

	order := &e.orders[slot]
	*order = Order{
		level: level,
		id:    oID,
		size:  size,
	}

	// Link into tail of the queue, using slots instead of OrderIDs
	if level.headSlot == 0 {
		level.headSlot = slot
	} else {
		tail := &e.orders[level.tailSlot]
		tail.nextSlot = slot
		order.prevSlot = level.tailSlot
	}
	level.tailSlot = slot
	level.size++

	// External ID -> internal slot
	e.orderIndex[oID] = slot
}

// Cancel order by removing from price level queue
func (e *MatchingEngine) Cancel(cancelOrderID OrderID) {
	// Validate order ID
	if cancelOrderID == 0 || cancelOrderID > e.orderID {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	cancelSlot := e.orderIndex[cancelOrderID]
	cancelOrder := &e.orders[cancelSlot]

	// Already filled, cancelled or recycled
	if cancelOrder.size == 0 || cancelSlot == 0 {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	e.unlink(cancelOrder.level, cancelSlot)
	cancelOrder.size = 0 // Mark as cancelled

	// Report order cancellation
	e.outputRing.Push(OutputEvent{
		eventType: CANCEL_EVENT,
		orderID:   cancelOrderID,
	})
}

// Remove order from doubly-linked list maintaining FIFO integrity
func (e *MatchingEngine) unlink(level *PriceLevel, slot Slot) {
	order := &e.orders[slot]

	if order.prevSlot != 0 {
		e.orders[order.prevSlot].nextSlot = order.nextSlot
	} else {
		level.headSlot = order.nextSlot // was the head
	}

	if order.nextSlot != 0 {
		e.orders[order.nextSlot].prevSlot = order.prevSlot
	} else {
		level.tailSlot = order.prevSlot // was the tail
	}

	level.size--
	e.orderIndex[order.id] = 0 // clear external -> slot mapping

	// Push into freeSlots ring if there's room
	nextTail := (e.freeTail + 1) & FREE_MASK
	if nextTail != (e.freeHead & FREE_MASK) {
		e.freeSlots[e.freeTail&FREE_MASK] = uint32(slot)
		e.freeTail++
	}

	order.prevSlot = 0
	order.nextSlot = 0
}
