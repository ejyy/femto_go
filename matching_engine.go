package main

const (
	MAX_SYMBOLS      = 1 << 8  // 256 trading symbols
	MAX_PRICE_LEVELS = 1 << 14 // 16,384 price ticks

	SLOT_BITS = 26
	SLOT_MASK = (1 << SLOT_BITS) - 1

	MAX_ORDERS = 1 << SLOT_BITS // 67M total orders
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

// Add a new limit order to the order book
func (e *MatchingEngine) Limit(symbol Symbol, side Side, price Price, size Size, trader TraderID) {
	if price == 0 || size == 0 || price >= MAX_PRICE_LEVELS {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	// Allocate a new order slot and generate a unique order ID
	slot, gen := e.pool.alloc()
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

	remaining := book.match(e.pool, e.outputRing, size, symbol, side, price, trader, newOrderID)

	if remaining > 0 {
		book.add(e.pool, side, price, newOrderID, slot, remaining)
	} else {
		e.pool.free(slot) // Free the slot if the order was fully matched
	}
}

func (e *MatchingEngine) Cancel(id OrderID) {
	// Extract the slot from the order ID
	slot := Slot(id & SLOT_MASK)

	if !e.pool.isValid(slot) {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	order := e.pool.get(slot)

	// Check if the order is valid and not already canceled
	if order.gen != Gen(id>>SLOT_BITS) || order.size == 0 {
		e.outputRing.Push(OutputEvent{eventType: REJECT_EVENT})
		return
	}

	order.level.remove(e.pool, slot)
	e.outputRing.Push(OutputEvent{eventType: CANCEL_EVENT, orderID: id})
}
