package main

const (
	MAX_SYMBOLS        = 1 << 8                 // 256 trading symbols
	MAX_PRICE_LEVELS   = 1 << 14                // 16,384 price ticks
	MAX_ORDERS         = 1 << 26                // 67M total orders
	DISTRIBUTOR_BUFFER = 1 << 10                // 1024 event size
	FREE_MASK          = DISTRIBUTOR_BUFFER - 1 // 1023 free slot mask
)

// Exchange engine with pre-allocated arrays
type Engine struct {
	books [MAX_SYMBOLS]OrderBook // Order books per symbol

	orders     [MAX_ORDERS]Order  // Pre-allocated order pool
	orderIndex [MAX_ORDERS]uint32 // Map of external OrderID -> internal slot index

	orderID OrderID // Monotonic order ID generator

	freeSlots [DISTRIBUTOR_BUFFER]uint32 // 'Recycled' Order slots
	freeHead  uint32                     // First free slot
	freeTail  uint32                     // Next empty slot

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
		e.addToBook(book, &order, side, price, newOrderID)
	}
}

// Match incoming order against opposite side of book
func (e *Engine) match(book *OrderBook, order *Order, oSymbol Symbol, oSide Side, oPrice Price, oTrader TraderID, oID OrderID) Size {
	remaining := order.Size

	if oSide == Bid {
		// Buy order matches against asks at or below bid price
		for remaining > 0 && book.askMin < MAX_PRICE_LEVELS && book.askMin <= oPrice {
			remaining = e.matchLevel(&book.askLevels[book.askMin], remaining, book.askMin, oSymbol, oTrader, oID)
			if remaining > 0 {
				book.updateBestAsk() // Find next best ask
			}
		}
	} else {
		// Sell order matches against bids at or above ask price
		for remaining > 0 && book.bidMax > 0 && book.bidMax >= oPrice {
			remaining = e.matchLevel(&book.bidLevels[book.bidMax], remaining, book.bidMax, oSymbol, oTrader, oID)
			if remaining > 0 {
				book.updateBestBid() // Find next best bid
			}
		}
	}

	return remaining
}

// Execute trades against orders at specific price level (FIFO)
func (e *Engine) matchLevel(level *PriceLevel, remaining Size, price Price, oSymbol Symbol, oTrader TraderID, oID OrderID) Size {
	for currentID := level.head; currentID != 0 && remaining > 0; {
		slot := e.orderIndex[currentID]
		counterOrder := &e.orders[slot]
		nextID := counterOrder.Next // Save before potential unlink

		fillSize := min(remaining, counterOrder.Size)

		// Report trade execution
		e.outputRing.Push(OutputEvent{
			Type:           EXECUTION_EVENT,
			OrderID:        oID,
			Price:          price, // Trade at resting order price
			Size:           fillSize,
			Trader:         oTrader,
			Symbol:         oSymbol,
			CounterOrderID: currentID,
		})

		remaining -= fillSize
		counterOrder.Size -= fillSize

		// Remove fully filled orders
		if counterOrder.Size == 0 {
			e.unlink(level, currentID)
		}

		currentID = nextID
	}

	return remaining
}

// Insert order into appropriate price level queue (FIFO)
func (e *Engine) addToBook(book *OrderBook, order *Order, oSide Side, oPrice Price, oID OrderID) {
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

	slot := e.orderIndex[oID]
	e.orders[slot] = *order
	level.size++
}

// Cancel order by removing from price level queue
func (e *Engine) Cancel(orderID OrderID) {
	// Validate order ID
	if orderID == 0 || orderID > e.orderID {
		e.outputRing.Push(OutputEvent{Type: REJECT_EVENT})
		return
	}

	slot := e.orderIndex[orderID]
	order := &e.orders[slot]

	// Already filled or cancelled
	if order.Size == 0 {
		e.outputRing.Push(OutputEvent{Type: REJECT_EVENT})
		return
	}

	e.unlink(order.Level, orderID)
	order.Size = 0 // Mark as cancelled

	// Push into freeSlots array if there is room
	nextTail := (e.freeTail + 1) & FREE_MASK
	if nextTail != (e.freeHead & FREE_MASK) {
		e.freeSlots[e.freeTail&FREE_MASK] = slot
		e.freeTail++
	}

	// Report order cancellation
	e.outputRing.Push(OutputEvent{
		Type:    CANCEL_EVENT,
		OrderID: orderID,
	})
}

// Remove order from doubly-linked list maintaining FIFO integrity
func (e *Engine) unlink(level *PriceLevel, orderID OrderID) {
	slot := e.orderIndex[orderID]
	order := &e.orders[slot]

	// Update previous order's next OrderID
	if order.Prev != 0 {
		prevSlot := e.orderIndex[order.Prev]
		e.orders[prevSlot].Next = order.Next
	} else {
		level.head = order.Next // This was the head
	}

	// Update next order's previous OrderID
	if order.Next != 0 {
		nextSlot := e.orderIndex[order.Next]
		e.orders[nextSlot].Prev = order.Prev
	} else {
		level.tail = order.Prev // This was the tail
	}

	// Clear references and decrement size
	order.Next = 0
	order.Prev = 0
	level.size--
}

// Distributes commands to engine
func (e *Engine) InputDistributor() {
	buf := make([]InputCommand, DISTRIBUTOR_BUFFER)
	for {
		n := e.inputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			ev := &buf[i]
			switch ev.Type {
			case ORDER_EVENT:
				e.Limit(ev.Symbol, ev.Side, ev.Price, ev.Size, ev.Trader)
			case CANCEL_EVENT:
				e.Cancel(ev.OrderID)
			}
		}
	}
}

// Distributes commands from engine
func (e *Engine) OutputDistributor(callbackFunc func(OutputEvent)) {
	buf := make([]OutputEvent, DISTRIBUTOR_BUFFER)
	for {
		n := e.outputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			callbackFunc(buf[i])
		}
	}
}
