package main

type OrderPool struct {
	orders       [MAX_ORDERS]Order
	freeHead     Slot // Head of the reusable free list (0 means empty)
	nextFreeSlot Slot // Highest slot allocated so far (0 means none allocated)
}

// Creates an empty pool (slot 0 is reserved as null slot)
func NewOrderPool() *OrderPool {
	return &OrderPool{}
}

// Returns a reusable slot when possible, otherwise allocates the next available slot
func (p *OrderPool) alloc() (Slot, Gen, bool) {
	var slot Slot
	if p.freeHead != 0 {
		slot = p.freeHead
		p.freeHead = p.orders[slot].nextSlot
	} else {
		if p.nextFreeSlot >= MAX_ORDERS-1 {
			return 0, 0, false
		}
		p.nextFreeSlot++
		slot = p.nextFreeSlot
	}
	return slot, p.orders[slot].gen, true
}

// Marks a slot unused and increments its generation (so that stale references can be detected)
func (p *OrderPool) free(slot Slot) {
	order := &p.orders[slot]
	order.gen++
	order.size = 0
	order.nextSlot = p.freeHead
	p.freeHead = slot
}

// Returns the order stored at a pool slot
func (p *OrderPool) get(slot Slot) *Order {
	return &p.orders[slot]
}

// Checks whether a slot has been allocated by the pool
func (p *OrderPool) isValid(slot Slot) bool {
	return slot != 0 && slot <= p.nextFreeSlot
}
