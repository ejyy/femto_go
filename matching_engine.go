package main

const (
	MAX_SYMBOLS      = 1 << 8  // Number of trading symbols (256)
	MAX_PRICE_LEVELS = 1 << 14 // Number of discrete price levels [ticks] (16,384)

	SLOT_BITS = 24
	SLOT_MASK = (1 << SLOT_BITS) - 1

	MAX_ORDERS = SLOT_MASK // Number of orderpool slots (16M)
)

type MatchingEngine struct {
	books [MAX_SYMBOLS]OrderBook
	pool  *OrderPool

	inputRing  *RingBuffer[InputCommand]
	outputRing *RingBuffer[OutputEvent]
}

func NewMatchingEngine() *MatchingEngine {
	e := &MatchingEngine{
		pool:       NewOrderPool(),
		inputRing:  NewRingBuffer[InputCommand](),
		outputRing: NewRingBuffer[OutputEvent](),
	}

	// Initialize order books for each symbol
	for i := range e.books {
		e.books[i] = OrderBook{askMin: MAX_PRICE_LEVELS, bidMax: 0}
	}
	return e
}

// Submits a new limit order, matching against the opposite side before adding unfilled quantity to the book
func (e *MatchingEngine) Limit(symbol Symbol, side Side, price Price, size Size, trader TraderID) {
	// Rejects malformed orders
	if price == 0 || size == 0 || price >= MAX_PRICE_LEVELS || symbol >= MAX_SYMBOLS {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT, orderID: 0, trader: trader})
		return
	}

	// Allocate a new order slot and generate a unique order ID
	slot, gen, ok := e.pool.alloc()
	if !ok {
		e.outputRing.Push(OutputEvent{eventType: ERROR_EVENT, orderID: 0, trader: trader})
		return
	}
	newOrderID := OrderID(uint64(gen)<<SLOT_BITS | uint64(slot))

	e.outputRing.Push(OutputEvent{
		eventType: ORDER_EVENT,
		orderID:   newOrderID,
		price:     price,
		size:      size,
		trader:    trader,
		symbol:    symbol,
		side:      side,
	})

	book := &e.books[symbol]

	// Match against existing orders
	remaining := book.match(e.pool, e.outputRing, size, symbol, side, price, trader, newOrderID)

	// Add any unfilled quantity to the book
	if remaining > 0 {
		book.add(e.pool, side, price, slot, remaining, symbol)
	} else {
		e.pool.free(slot) // Free the slot as the order was fully matched
	}
}

// Removes a live order
func (e *MatchingEngine) Cancel(id OrderID) {
	// Extract the slot from the order ID
	slot := Slot(id & SLOT_MASK)

	if !e.pool.isValid(slot) {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT, orderID: id})
		return
	}

	order := e.pool.get(slot)

	// Check if the order is valid and not already canceled
	if order.gen != Gen(id>>SLOT_BITS) || order.size == 0 {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT, orderID: id})
		return
	}

	book := &e.books[order.symbol]

	// Remove order from its price level and return slot to pool
	level := book.level(order.side, order.price)
	level.remove(e.pool, slot)

	e.outputRing.Push(OutputEvent{eventType: CANCEL_EVENT, orderID: id})
}
