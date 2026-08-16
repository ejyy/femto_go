package main

type OrderPool struct {
	orders       [MAX_ORDERS]Order
	freeHead     Slot // Head of the free list (0 means empty)
	nextFreeSlot Slot // Next slot to allocate if free list is empty
}

func NewOrderPool() *OrderPool {
	return &OrderPool{}
}

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

func (p *OrderPool) free(slot Slot) {
	order := &p.orders[slot]
	order.gen++
	order.size = 0
	order.nextSlot = p.freeHead
	p.freeHead = slot
}

func (p *OrderPool) get(slot Slot) *Order {
	return &p.orders[slot]
}

func (p *OrderPool) isValid(slot Slot) bool {
	return slot != 0 && slot <= p.nextFreeSlot
}
