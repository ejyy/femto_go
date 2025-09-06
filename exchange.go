package main

const (
	MAX_SYMBOLS      = 1 << 8         // 256 trading symbols
	MAX_PRICE_LEVELS = 1 << 14        // 16,384 price ticks
	MAX_ORDERS       = 1 << 26        // 67M total orders
	FREE_SLOTS       = 1 << 10        // 1024 free order slots
	FREE_MASK        = FREE_SLOTS - 1 // Free slot mask
)

// Exchange engine with pre-allocated arrays
type Engine struct {
	books [MAX_SYMBOLS]OrderBook // Order books per symbol

	orders     [MAX_ORDERS]Order  // Pre-allocated order pool
	orderIndex [MAX_ORDERS]uint32 // External OrderID -> internal slot index

	orderID OrderID // Monotonic order ID generator

	freeSlots [FREE_SLOTS]uint32 // 'Recycled' Order slots
	freeHead  uint32             // First free slot
	freeTail  uint32             // Next empty slot

	inputRing  *RingBuffer[InputCommand] // Incoming commands
	outputRing *RingBuffer[OutputEvent]  // Outgoing events
}

func NewEngine() *Engine {
	e := &Engine{
		inputRing:  NewRingBuffer[InputCommand](RING_SIZE),
		outputRing: NewRingBuffer[OutputEvent](RING_SIZE),
	}

	// Set  ask minimum to initial value (no asks)
	for i := range e.books {
		e.books[i] = OrderBook{
			askMin: MAX_PRICE_LEVELS,
			bidMax: 0,
		}
	}

	return e
}

// Process limit order with matching and book insertion
func (e *Engine) Limit(symbol Symbol, side Side, price Price, size Size, trader TraderID) {
	// Validate order parameters
	if price == 0 || size == 0 || price >= MAX_PRICE_LEVELS {
		e.outputRing.Push(OutputEvent{Type: REJECT_EVENT})
		return
	}

	e.orderID++
	newOrderID := e.orderID

	// Pick internal slot (recycled or new)
	var slot uint32
	if e.freeHead != e.freeTail {
		slot = e.freeSlots[e.freeHead&FREE_MASK]
		e.freeHead++
	} else {
		slot = uint32(newOrderID)
	}

	e.orderIndex[newOrderID] = slot

	// Build Order object based on function parameters
	order := Order{
		Size: size,
	}

	// Report order receipt
	e.outputRing.Push(OutputEvent{
		Type:    ORDER_EVENT,
		OrderID: newOrderID,
		Price:   price,
		Size:    size,
		Trader:  trader,
		Symbol:  symbol,
		Side:    side,
	})

	// Lookup and match according to symbol
	book := &e.books[symbol]
	remaining := e.match(book, &order, symbol, side, price, trader, newOrderID)

	// Add unfilled portion to book
	if remaining > 0 {
		order.Size = remaining
		e.addToBook(book, &order, side, price, newOrderID, slot)
	}
}

// Match incoming order against opposite side of book
//
//go:inline
func (e *Engine) match(book *OrderBook, order *Order, oSymbol Symbol, oSide Side, oPrice Price, oTrader TraderID, oID OrderID) (remaining Size) {
	remaining = order.Size

	if oSide == Bid {
		// Buy order matches against asks at or below bid price
		for remaining > 0 && book.askMin < MAX_PRICE_LEVELS && book.askMin <= oPrice {
			remaining = e.matchLevel(&book.askLevels[book.askMin], remaining, book.askMin, oSymbol, oTrader, oID)
			if remaining > 0 && book.askLevels[book.askMin].head == 0 { // Only checks if PriceLevel exhausted
				book.updateBestAsk() // Find next best ask
			}
		}
	} else {
		// Sell order matches against bids at or above ask price
		for remaining > 0 && book.bidMax > 0 && book.bidMax >= oPrice {
			remaining = e.matchLevel(&book.bidLevels[book.bidMax], remaining, book.bidMax, oSymbol, oTrader, oID)
			if remaining > 0 && book.bidLevels[book.bidMax].head == 0 { // Only checks if PriceLevel exhausted
				book.updateBestBid() // Find next best bid
			}
		}
	}

	return remaining
}

// Execute trades against orders at specific price level (FIFO)
//
//go:inline
func (e *Engine) matchLevel(level *PriceLevel, remaining Size, price Price, oSymbol Symbol, oTrader TraderID, oID OrderID) Size {
	for counterID := level.head; counterID != 0 && remaining > 0; {
		counterSlot := e.orderIndex[counterID]
		counterOrder := &e.orders[counterSlot]
		nextCounterID := counterOrder.Next // Save before potential unlink

		fillSize := min(remaining, counterOrder.Size)

		// Report trade execution
		e.outputRing.Push(OutputEvent{
			Type:           EXECUTION_EVENT,
			OrderID:        oID,
			Price:          price, // Trade at resting order price
			Size:           fillSize,
			Trader:         oTrader,
			Symbol:         oSymbol,
			CounterOrderID: counterID,
		})

		remaining -= fillSize
		counterOrder.Size -= fillSize

		// Remove fully filled orders
		if counterOrder.Size == 0 {
			e.unlink(level, counterID, counterSlot)
		}

		counterID = nextCounterID
	}

	return remaining
}

// Insert order into appropriate price level queue (FIFO)
//
//go:inline
func (e *Engine) addToBook(book *OrderBook, order *Order, oSide Side, oPrice Price, oID OrderID, slot uint32) {
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

	order.Level = level

	// Initialize empty level or append to tail
	if level.head == 0 {
		level.head = oID
		level.tail = oID
	} else {
		tailSlot := e.orderIndex[level.tail]
		tail := &e.orders[tailSlot]
		tail.Next = oID
		order.Prev = level.tail
		level.tail = oID
	}

	e.orders[slot] = *order
	level.size++
}

// Cancel order by removing from price level queue
func (e *Engine) Cancel(cancelOrderID OrderID) {
	// Validate order ID
	if cancelOrderID == 0 || cancelOrderID > e.orderID {
		e.outputRing.Push(OutputEvent{Type: REJECT_EVENT})
		return
	}

	cancelSlot := e.orderIndex[cancelOrderID]
	cancelOrder := &e.orders[cancelSlot]

	// Already filled, cancelled or recycled
	if cancelOrder.Size == 0 || cancelSlot == 0 {
		e.outputRing.Push(OutputEvent{Type: REJECT_EVENT})
		return
	}

	e.unlink(cancelOrder.Level, cancelOrderID, cancelSlot)
	cancelOrder.Size = 0 // Mark as cancelled

	// Report order cancellation
	e.outputRing.Push(OutputEvent{
		Type:    CANCEL_EVENT,
		OrderID: cancelOrderID,
	})
}

// Remove order from doubly-linked list maintaining FIFO integrity
//
//go:inline
func (e *Engine) unlink(level *PriceLevel, unlinkOrderID OrderID, unlinkSlot uint32) {
	unlinkOrder := &e.orders[unlinkSlot]

	// Update previous order's next OrderID
	if unlinkOrder.Prev != 0 {
		prevSlot := e.orderIndex[unlinkOrder.Prev]
		e.orders[prevSlot].Next = unlinkOrder.Next
	} else {
		level.head = unlinkOrder.Next // This was the head
	}

	// Update next order's previous OrderID
	if unlinkOrder.Next != 0 {
		nextSlot := e.orderIndex[unlinkOrder.Next]
		e.orders[nextSlot].Prev = unlinkOrder.Prev
	} else {
		level.tail = unlinkOrder.Prev // This was the tail
	}

	// Push into freeSlots array if there is room
	nextTail := (e.freeTail + 1) & FREE_MASK
	if nextTail != (e.freeHead & FREE_MASK) {
		e.freeSlots[e.freeTail&FREE_MASK] = unlinkSlot
		e.freeTail++
	}

	// Clear references and decrement size
	unlinkOrder.Next = 0
	unlinkOrder.Prev = 0
	level.size--
	e.orderIndex[unlinkOrderID] = 0
}
