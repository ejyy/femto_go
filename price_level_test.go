package main

import "testing"

func TestPriceLevelPushBackEmpty(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot, _, _ := pool.alloc()
	level.pushBack(pool, slot)

	if level.headSlot != slot {
		t.Fatalf("headSlot = %d, want %d", level.headSlot, slot)
	}

	if level.tailSlot != slot {
		t.Fatalf("tailSlot = %d, want %d", level.tailSlot, slot)
	}

	order := pool.get(slot)

	if order.prevSlot != 0 {
		t.Errorf("order.prevSlot = %d, want 0", order.prevSlot)
	}

	if order.nextSlot != 0 {
		t.Errorf("order.nextSlot = %d, want 0", order.nextSlot)
	}
}

func TestPriceLevelPushBackMaintainsFIFO(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()
	slot3, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)
	level.pushBack(pool, slot3)

	if level.headSlot != slot1 {
		t.Fatalf("headSlot = %d, want %d", level.headSlot, slot1)
	}

	if level.tailSlot != slot3 {
		t.Fatalf("tailSlot = %d, want %d", level.tailSlot, slot3)
	}

	order1 := pool.get(slot1)
	order2 := pool.get(slot2)
	order3 := pool.get(slot3)

	if order1.prevSlot != 0 {
		t.Errorf("order1.prevSlot = %d, want 0", order1.prevSlot)
	}

	if order1.nextSlot != slot2 {
		t.Errorf("order1.nextSlot = %d, want %d", order1.nextSlot, slot2)
	}

	if order2.prevSlot != slot1 {
		t.Errorf("order2.prevSlot = %d, want %d", order2.prevSlot, slot1)
	}

	if order2.nextSlot != slot3 {
		t.Errorf("order2.nextSlot = %d, want %d", order2.nextSlot, slot3)
	}

	if order3.prevSlot != slot2 {
		t.Errorf("order3.prevSlot = %d, want %d", order3.prevSlot, slot2)
	}

	if order3.nextSlot != 0 {
		t.Errorf("order3.nextSlot = %d, want 0", order3.nextSlot)
	}
}

func TestPriceLevelPushBackResetsLinks(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()
	slot3, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)

	// Give slot3 stale links before inserting it.
	order3 := pool.get(slot3)
	order3.prevSlot = 123
	order3.nextSlot = 456

	level.pushBack(pool, slot3)

	if order3.prevSlot != slot2 {
		t.Errorf("order3.prevSlot = %d, want %d", order3.prevSlot, slot2)
	}

	if order3.nextSlot != 0 {
		t.Errorf("order3.nextSlot = %d, want 0", order3.nextSlot)
	}
}

func TestPriceLevelRemoveOnlyOrder(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot, _, _ := pool.alloc()
	level.pushBack(pool, slot)

	level.remove(pool, slot)

	if level.headSlot != 0 {
		t.Errorf("headSlot = %d, want 0", level.headSlot)
	}

	if level.tailSlot != 0 {
		t.Errorf("tailSlot = %d, want 0", level.tailSlot)
	}

	if pool.freeHead != slot {
		t.Errorf("freeHead = %d, want %d", pool.freeHead, slot)
	}
}

func TestPriceLevelRemoveHead(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()
	slot3, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)
	level.pushBack(pool, slot3)

	level.remove(pool, slot1)

	if level.headSlot != slot2 {
		t.Fatalf("headSlot = %d, want %d", level.headSlot, slot2)
	}

	if level.tailSlot != slot3 {
		t.Fatalf("tailSlot = %d, want %d", level.tailSlot, slot3)
	}

	order2 := pool.get(slot2)
	order3 := pool.get(slot3)

	if order2.prevSlot != 0 {
		t.Errorf("order2.prevSlot = %d, want 0", order2.prevSlot)
	}

	if order2.nextSlot != slot3 {
		t.Errorf("order2.nextSlot = %d, want %d", order2.nextSlot, slot3)
	}

	if order3.prevSlot != slot2 {
		t.Errorf("order3.prevSlot = %d, want %d", order3.prevSlot, slot2)
	}

	if order3.nextSlot != 0 {
		t.Errorf("order3.nextSlot = %d, want 0", order3.nextSlot)
	}
}

func TestPriceLevelRemoveTail(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()
	slot3, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)
	level.pushBack(pool, slot3)

	level.remove(pool, slot3)

	if level.headSlot != slot1 {
		t.Fatalf("headSlot = %d, want %d", level.headSlot, slot1)
	}

	if level.tailSlot != slot2 {
		t.Fatalf("tailSlot = %d, want %d", level.tailSlot, slot2)
	}

	order1 := pool.get(slot1)
	order2 := pool.get(slot2)

	if order1.prevSlot != 0 {
		t.Errorf("order1.prevSlot = %d, want 0", order1.prevSlot)
	}

	if order1.nextSlot != slot2 {
		t.Errorf("order1.nextSlot = %d, want %d", order1.nextSlot, slot2)
	}

	if order2.prevSlot != slot1 {
		t.Errorf("order2.prevSlot = %d, want %d", order2.prevSlot, slot1)
	}

	if order2.nextSlot != 0 {
		t.Errorf("order2.nextSlot = %d, want 0", order2.nextSlot)
	}
}

func TestPriceLevelRemoveMiddle(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()
	slot3, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)
	level.pushBack(pool, slot3)

	level.remove(pool, slot2)

	if level.headSlot != slot1 {
		t.Fatalf("headSlot = %d, want %d", level.headSlot, slot1)
	}

	if level.tailSlot != slot3 {
		t.Fatalf("tailSlot = %d, want %d", level.tailSlot, slot3)
	}

	order1 := pool.get(slot1)
	order3 := pool.get(slot3)

	if order1.prevSlot != 0 {
		t.Errorf("order1.prevSlot = %d, want 0", order1.prevSlot)
	}

	if order1.nextSlot != slot3 {
		t.Errorf("order1.nextSlot = %d, want %d", order1.nextSlot, slot3)
	}

	if order3.prevSlot != slot1 {
		t.Errorf("order3.prevSlot = %d, want %d", order3.prevSlot, slot1)
	}

	if order3.nextSlot != 0 {
		t.Errorf("order3.nextSlot = %d, want 0", order3.nextSlot)
	}
}

func TestPriceLevelRemoveReturnsSlotToPool(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)

	level.remove(pool, slot1)

	if pool.freeHead != slot1 {
		t.Fatalf("freeHead = %d, want %d", pool.freeHead, slot1)
	}

	// The next allocation should reuse the freed slot.
	reused, gen, _ := pool.alloc()

	if reused != slot1 {
		t.Fatalf("reused slot = %d, want %d", reused, slot1)
	}

	if gen != 1 {
		t.Fatalf("reused generation = %d, want 1", gen)
	}
}

func TestPriceLevelRemoveMultipleOrders(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slots := make([]Slot, 5)

	for i := range slots {
		slots[i], _, _ = pool.alloc()
		level.pushBack(pool, slots[i])
	}

	// Remove in this order:
	// head, middle, tail, remaining head, remaining tail.
	level.remove(pool, slots[0])

	if level.headSlot != slots[1] {
		t.Fatalf("after removing slot0: headSlot = %d, want %d",
			level.headSlot, slots[1])
	}

	level.remove(pool, slots[2])

	if pool.get(slots[1]).nextSlot != slots[3] {
		t.Fatalf("slot1.nextSlot = %d, want %d",
			pool.get(slots[1]).nextSlot, slots[3])
	}

	if pool.get(slots[3]).prevSlot != slots[1] {
		t.Fatalf("slot3.prevSlot = %d, want %d",
			pool.get(slots[3]).prevSlot, slots[1])
	}

	level.remove(pool, slots[4])

	if level.tailSlot != slots[3] {
		t.Fatalf("after removing slot4: tailSlot = %d, want %d",
			level.tailSlot, slots[3])
	}

	level.remove(pool, slots[1])

	if level.headSlot != slots[3] {
		t.Fatalf("after removing slot1: headSlot = %d, want %d",
			level.headSlot, slots[3])
	}

	if level.tailSlot != slots[3] {
		t.Fatalf("after removing slot1: tailSlot = %d, want %d",
			level.tailSlot, slots[3])
	}

	level.remove(pool, slots[3])

	if level.headSlot != 0 {
		t.Errorf("final headSlot = %d, want 0", level.headSlot)
	}

	if level.tailSlot != 0 {
		t.Errorf("final tailSlot = %d, want 0", level.tailSlot)
	}
}

func TestPriceLevelFIFOAfterRemoval(t *testing.T) {
	pool := NewOrderPool()
	level := PriceLevel{}

	slot1, _, _ := pool.alloc()
	slot2, _, _ := pool.alloc()
	slot3, _, _ := pool.alloc()
	slot4, _, _ := pool.alloc()

	level.pushBack(pool, slot1)
	level.pushBack(pool, slot2)
	level.pushBack(pool, slot3)
	level.pushBack(pool, slot4)

	// Remove the second order.
	level.remove(pool, slot2)

	// Expected queue:
	// slot1 -> slot3 -> slot4
	if level.headSlot != slot1 {
		t.Fatalf("headSlot = %d, want %d", level.headSlot, slot1)
	}

	if pool.get(slot1).nextSlot != slot3 {
		t.Fatalf("slot1.nextSlot = %d, want %d",
			pool.get(slot1).nextSlot, slot3)
	}

	if pool.get(slot3).prevSlot != slot1 {
		t.Fatalf("slot3.prevSlot = %d, want %d",
			pool.get(slot3).prevSlot, slot1)
	}

	if pool.get(slot3).nextSlot != slot4 {
		t.Fatalf("slot3.nextSlot = %d, want %d",
			pool.get(slot3).nextSlot, slot4)
	}

	if pool.get(slot4).prevSlot != slot3 {
		t.Fatalf("slot4.prevSlot = %d, want %d",
			pool.get(slot4).prevSlot, slot3)
	}

	if level.tailSlot != slot4 {
		t.Fatalf("tailSlot = %d, want %d", level.tailSlot, slot4)
	}
}
