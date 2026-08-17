package main

// A FIFO queue of orders at a specific price
type PriceLevel struct {
	headSlot Slot // First order (oldest)
	tailSlot Slot // Last order (newest)
}

// Adds a new order to the end of the FIFO queue at this price level
func (level *PriceLevel) pushBack(pool *OrderPool, slot Slot) {
	order := pool.get(slot)

	// Resets links in case the slot was previously used
	order.prevSlot = 0
	order.nextSlot = 0

	if level.headSlot == 0 {
		// First order at this price level
		level.headSlot = slot
	} else {
		// Link the current tail to the new order
		tail := pool.get(level.tailSlot)
		tail.nextSlot = slot
		order.prevSlot = level.tailSlot
	}
	level.tailSlot = slot
}

// Unlinks an order and returns it to the free slot pool
func (level *PriceLevel) remove(pool *OrderPool, slot Slot) {
	order := pool.get(slot)

	// Update the previous order (or move head if removing first order)
	if order.prevSlot != 0 {
		pool.get(order.prevSlot).nextSlot = order.nextSlot
	} else {
		level.headSlot = order.nextSlot
	}

	// Update the next order (or move tail if removing last order)
	if order.nextSlot != 0 {
		pool.get(order.nextSlot).prevSlot = order.prevSlot
	} else {
		level.tailSlot = order.prevSlot
	}

	pool.free(slot)
}
