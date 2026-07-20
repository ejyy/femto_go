package main

// Slot constants.
const (
	SLOT_BITS = 26
	SLOT_MASK = (1 << SLOT_BITS) - 1
)

// Pack slot + generation into an OrderID.
func NewOrderID(slot Slot, gen Gen) OrderID {
	return OrderID(uint64(gen)<<SLOT_BITS | uint64(slot))
}

// Decode OrderID.
func (id OrderID) slot() Slot {
	return Slot(id & SLOT_MASK)
}

func (id OrderID) gen() Gen {
	return Gen(id >> SLOT_BITS)
}

// Allocate a slot from the free list or extend the pool.
func (e *MatchingEngine) allocSlot() (Slot, Gen) {
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

// Return a slot to the free list.
func (e *MatchingEngine) freeSlot(slot Slot) {
	order := &e.orders[slot]

	order.gen++
	order.nextSlot = e.freeHead
	e.freeHead = slot
}

// Has this slot ever been allocated?
func (e *MatchingEngine) validSlot(slot Slot) bool {
	return slot != 0 && slot <= e.nextFreeSlot
}
