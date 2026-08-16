package main

import "testing"

func TestNewOrderPool(t *testing.T) {
	p := NewOrderPool()

	if p == nil {
		t.Fatal("NewOrderPool() returned nil")
	}

	if p.freeHead != 0 {
		t.Fatalf("freeHead = %d, want 0", p.freeHead)
	}

	if p.nextFreeSlot != 0 {
		t.Fatalf("nextFreeSlot = %d, want 0", p.nextFreeSlot)
	}
}

func TestOrderPoolAllocSequential(t *testing.T) {
	p := NewOrderPool()

	tests := []struct {
		wantSlot Slot
		wantGen  Gen
	}{
		{1, 0},
		{2, 0},
		{3, 0},
	}

	for i, tt := range tests {
		slot, gen, _ := p.alloc()

		if slot != tt.wantSlot {
			t.Errorf("alloc #%d: slot = %d, want %d", i+1, slot, tt.wantSlot)
		}

		if gen != tt.wantGen {
			t.Errorf("alloc #%d: gen = %d, want %d", i+1, gen, tt.wantGen)
		}
	}

	if p.nextFreeSlot != 3 {
		t.Fatalf("nextFreeSlot = %d, want 3", p.nextFreeSlot)
	}

	if p.freeHead != 0 {
		t.Fatalf("freeHead = %d, want 0", p.freeHead)
	}
}

func TestOrderPoolAllocFreeReusesSlot(t *testing.T) {
	p := NewOrderPool()

	slot1, gen1, _ := p.alloc()
	if slot1 != 1 || gen1 != 0 {
		t.Fatalf("first alloc = (%d, %d), want (1, 0)", slot1, gen1)
	}

	p.free(slot1)

	if p.freeHead != slot1 {
		t.Fatalf("freeHead = %d, want %d", p.freeHead, slot1)
	}

	slot2, gen2, _ := p.alloc()

	if slot2 != slot1 {
		t.Fatalf("reused slot = %d, want %d", slot2, slot1)
	}

	if gen2 != 1 {
		t.Fatalf("reused slot generation = %d, want 1", gen2)
	}

	if p.freeHead != 0 {
		t.Fatalf("freeHead = %d, want 0", p.freeHead)
	}

	if p.nextFreeSlot != 1 {
		t.Fatalf("nextFreeSlot = %d, want 1", p.nextFreeSlot)
	}
}

func TestOrderPoolFreeIncrementsGeneration(t *testing.T) {
	p := NewOrderPool()

	slot, gen, _ := p.alloc()

	if gen != 0 {
		t.Fatalf("initial generation = %d, want 0", gen)
	}

	p.free(slot)

	if got := p.orders[slot].gen; got != 1 {
		t.Fatalf("generation after free = %d, want 1", got)
	}

	p.alloc()
	p.free(slot)

	if got := p.orders[slot].gen; got != 2 {
		t.Fatalf("generation after second free = %d, want 2", got)
	}
}

func TestOrderPoolFreeResetsSize(t *testing.T) {
	p := NewOrderPool()

	slot, _, _ := p.alloc()
	p.orders[slot].size = 12345

	p.free(slot)

	if got := p.orders[slot].size; got != 0 {
		t.Fatalf("size after free = %d, want 0", got)
	}
}

func TestOrderPoolFreeListIsLIFO(t *testing.T) {
	p := NewOrderPool()

	slot1, _, _ := p.alloc()
	slot2, _, _ := p.alloc()
	slot3, _, _ := p.alloc()

	p.free(slot1)
	p.free(slot2)
	p.free(slot3)

	// Free list should be:
	// slot3 -> slot2 -> slot1
	if p.freeHead != slot3 {
		t.Fatalf("freeHead = %d, want %d", p.freeHead, slot3)
	}

	got, _, _ := p.alloc()
	if got != slot3 {
		t.Fatalf("first reused slot = %d, want %d", got, slot3)
	}

	got, _, _ = p.alloc()
	if got != slot2 {
		t.Fatalf("second reused slot = %d, want %d", got, slot2)
	}

	got, _, _ = p.alloc()
	if got != slot1 {
		t.Fatalf("third reused slot = %d, want %d", got, slot1)
	}
}

func TestOrderPoolFreeListPreservesNextLinks(t *testing.T) {
	p := NewOrderPool()

	slot1, _, _ := p.alloc()
	slot2, _, _ := p.alloc()
	slot3, _, _ := p.alloc()

	p.free(slot1)
	p.free(slot2)
	p.free(slot3)

	if got := p.orders[slot3].nextSlot; got != slot2 {
		t.Fatalf("slot3.nextSlot = %d, want %d", got, slot2)
	}

	if got := p.orders[slot2].nextSlot; got != slot1 {
		t.Fatalf("slot2.nextSlot = %d, want %d", got, slot1)
	}

	if got := p.orders[slot1].nextSlot; got != 0 {
		t.Fatalf("slot1.nextSlot = %d, want 0", got)
	}
}

func TestOrderPoolAllocFromFreeListDoesNotAdvanceNextFreeSlot(t *testing.T) {
	p := NewOrderPool()

	slot1, _, _ := p.alloc()
	slot2, _, _ := p.alloc()

	p.free(slot1)
	p.free(slot2)

	if p.nextFreeSlot != 2 {
		t.Fatalf("nextFreeSlot before reuse = %d, want 2", p.nextFreeSlot)
	}

	p.alloc()
	p.alloc()

	if p.nextFreeSlot != 2 {
		t.Fatalf("nextFreeSlot after reuse = %d, want 2", p.nextFreeSlot)
	}

	slot3, _, _ := p.alloc()

	if slot3 != 3 {
		t.Fatalf("new slot = %d, want 3", slot3)
	}

	if p.nextFreeSlot != 3 {
		t.Fatalf("nextFreeSlot = %d, want 3", p.nextFreeSlot)
	}
}

func TestOrderPoolGet(t *testing.T) {
	p := NewOrderPool()

	slot, _, _ := p.alloc()

	p.orders[slot].price = 100
	p.orders[slot].size = 25
	p.orders[slot].symbol = 42
	p.orders[slot].side = Bid

	got := p.get(slot)

	if got != &p.orders[slot] {
		t.Fatal("get() did not return pointer to the expected order")
	}

	if got.price != 100 {
		t.Errorf("price = %d, want 100", got.price)
	}

	if got.size != 25 {
		t.Errorf("size = %d, want 25", got.size)
	}

	if got.symbol != 42 {
		t.Errorf("symbol = %d, want 42", got.symbol)
	}

	if got.side != Bid {
		t.Errorf("side = %d, want %d", got.side, Bid)
	}
}

func TestOrderPoolIsValid(t *testing.T) {
	p := NewOrderPool()

	tests := []struct {
		name string
		slot Slot
		want bool
	}{
		{"zero slot", 0, false},
		{"before allocation", 1, false},
		{"arbitrary unallocated slot", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isValid(tt.slot); got != tt.want {
				t.Errorf("isValid(%d) = %v, want %v", tt.slot, got, tt.want)
			}
		})
	}

	slot1, _, _ := p.alloc()
	slot2, _, _ := p.alloc()

	tests = []struct {
		name string
		slot Slot
		want bool
	}{
		{"first allocated slot", slot1, true},
		{"second allocated slot", slot2, true},
		{"zero slot", 0, false},
		{"slot beyond allocation", 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.isValid(tt.slot); got != tt.want {
				t.Errorf("isValid(%d) = %v, want %v", tt.slot, got, tt.want)
			}
		})
	}
}

func TestOrderPoolFreedSlotRemainsValid(t *testing.T) {
	p := NewOrderPool()

	slot, _, _ := p.alloc()
	p.free(slot)

	if !p.isValid(slot) {
		t.Fatalf("isValid(%d) = false after free, want true", slot)
	}
}

func TestOrderPoolGenerationChangesOnReuse(t *testing.T) {
	p := NewOrderPool()

	slot, gen, _ := p.alloc()

	const cycles = 10

	for i := Gen(1); i <= cycles; i++ {
		p.free(slot)

		slot2, gen2, _ := p.alloc()

		if slot2 != slot {
			t.Fatalf("cycle %d: slot = %d, want %d", i, slot2, slot)
		}

		if gen2 != gen+i {
			t.Fatalf(
				"cycle %d: generation = %d, want %d",
				i,
				gen2,
				gen+i,
			)
		}
	}
}

func TestOrderPoolFreeDoesNotModifyOtherOrders(t *testing.T) {
	p := NewOrderPool()

	slot1, _, _ := p.alloc()
	slot2, _, _ := p.alloc()

	p.orders[slot1].price = 100
	p.orders[slot1].size = 10

	p.orders[slot2].price = 200
	p.orders[slot2].size = 20

	p.free(slot1)

	if p.orders[slot2].price != 200 {
		t.Errorf("slot2 price = %d, want 200", p.orders[slot2].price)
	}

	if p.orders[slot2].size != 20 {
		t.Errorf("slot2 size = %d, want 20", p.orders[slot2].size)
	}

	if p.orders[slot1].size != 0 {
		t.Errorf("slot1 size = %d after free, want 0", p.orders[slot1].size)
	}
}

func TestOrderPoolAllocInitializesOnlyGeneration(t *testing.T) {
	p := NewOrderPool()

	slot, gen, _ := p.alloc()

	if gen != 0 {
		t.Fatalf("generation = %d, want 0", gen)
	}

	order := p.get(slot)

	if order.gen != 0 {
		t.Errorf("order.gen = %d, want 0", order.gen)
	}
}
