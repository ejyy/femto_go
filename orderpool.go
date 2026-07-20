package main

type OrderPool struct {
	orders       [MAX_ORDERS]Order
	freeHead     Slot
	nextFreeSlot Slot
}

func NewOrderPool() *OrderPool {
	return &OrderPool{}
}

func (p *OrderPool) alloc() (Slot, Gen) {
	var slot Slot
	if p.freeHead != 0 {
		slot = p.freeHead
		p.freeHead = p.orders[slot].nextSlot
	} else {
		p.nextFreeSlot++
		slot = p.nextFreeSlot
	}
	return slot, p.orders[slot].gen
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
