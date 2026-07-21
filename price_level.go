package main

// Pricelevel serving as a FIFO queue of orders at a specific price
type PriceLevel struct {
	headSlot Slot // First order (oldest)
	tailSlot Slot // Last order (newest)
}

// pushBack adds a new order to the tail of this price level
func (level *PriceLevel) pushBack(pool *OrderPool, slot Slot) {
	order := pool.get(slot)

	order.prevSlot = 0
	order.nextSlot = 0

	if level.headSlot == 0 {
		level.headSlot = slot
	} else {
		tail := pool.get(level.tailSlot)
		tail.nextSlot = slot
		order.prevSlot = level.tailSlot
	}
	level.tailSlot = slot
}

// remove unlinks an order and returns it to the free pool
func (level *PriceLevel) remove(pool *OrderPool, slot Slot) {
	order := pool.get(slot)

	if order.prevSlot != 0 {
		pool.get(order.prevSlot).nextSlot = order.nextSlot
	} else {
		level.headSlot = order.nextSlot
	}

	if order.nextSlot != 0 {
		pool.get(order.nextSlot).prevSlot = order.prevSlot
	} else {
		level.tailSlot = order.prevSlot
	}

	pool.free(slot)
}
