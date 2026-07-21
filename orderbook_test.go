package main

import "testing"

// Helper to create a price level with a given number of orders
func makePriceLevel(size uint32) PriceLevel {
	if size == 0 {
		return PriceLevel{}
	}
	return PriceLevel{
		headSlot: Slot(1),    // Dummy OrderID
		tailSlot: Slot(size), // Dummy OrderID
	}
}

func TestUpdateBestBidEmptyBook(t *testing.T) {
	book := &OrderBook{
		bidMax: 15, // Random value
	}

	// No bid levels populated
	book.updateBidMax()
	if book.bidMax != 0 {
		t.Errorf("expected bidMax 0 for empty book, got %d", book.bidMax)
	}
}

func TestUpdateBestBid_SinglePriceLevel(t *testing.T) {
	book := &OrderBook{
		bidMax: 10,
	}
	book.bidLevels[10] = makePriceLevel(3)

	// Nothing else, updateBidMax should stay at 10
	book.updateBidMax()
	if book.bidMax != 10 {
		t.Errorf("expected bidMax 10, got %d", book.bidMax)
	}
}

func TestUpdateBestBid_MultipleLevels(t *testing.T) {
	book := &OrderBook{
		bidMax: 10,
	}
	book.bidLevels[10] = makePriceLevel(3)
	book.bidLevels[9] = makePriceLevel(2)
	book.bidLevels[7] = makePriceLevel(1)

	// Clear 10 (ie. all executed at that level), should move to 9
	book.bidLevels[10] = PriceLevel{}
	book.updateBidMax()
	if book.bidMax != 9 {
		t.Errorf("expected bidMax 9, got %d", book.bidMax)
	}

	// Clear 9, should move to 7
	book.bidLevels[9] = PriceLevel{}
	book.updateBidMax()
	if book.bidMax != 7 {
		t.Errorf("expected bidMax 7, got %d", book.bidMax)
	}

	// Clear 7, should reset to 0
	book.bidLevels[7] = PriceLevel{}
	book.updateBidMax()
	if book.bidMax != 0 {
		t.Errorf("expected bidMax 0, got %d", book.bidMax)
	}
}

func TestUpdateBestBid_Exhaustive(t *testing.T) {
	book := &OrderBook{}

	// Single bid at 10
	book.bidMax = 10
	book.bidLevels[10] = makePriceLevel(2)
	book.updateBidMax()
	if book.bidMax != 10 {
		t.Errorf("expected bidMax 10, got %d", book.bidMax)
	}

	// Multiple levels: 10, 9, 7
	book.bidLevels[9] = makePriceLevel(1)
	book.bidLevels[7] = makePriceLevel(3)

	// Clear 10 -> should move to 9
	book.bidLevels[10] = PriceLevel{}
	book.updateBidMax()
	if book.bidMax != 9 {
		t.Errorf("expected bidMax 9, got %d", book.bidMax)
	}

	// Clear 9 -> should move to 7
	book.bidLevels[9] = PriceLevel{}
	book.updateBidMax()
	if book.bidMax != 7 {
		t.Errorf("expected bidMax 7, got %d", book.bidMax)
	}

	// Clear 7 -> empty book
	book.bidLevels[7] = PriceLevel{}
	book.updateBidMax()
	if book.bidMax != 0 {
		t.Errorf("expected bidMax 0 for empty book, got %d", book.bidMax)
	}

	// Edge case: bid at price 0
	book.bidLevels[0] = makePriceLevel(1)
	book.bidMax = 0
	book.updateBidMax()
	if book.bidMax != 0 {
		t.Errorf("expected bidMax 0 for level 0, got %d", book.bidMax)
	}
}

func TestUpdateAskMinEmptyBook(t *testing.T) {
	book := &OrderBook{
		askMin: 5, // Random value
	}

	// No ask levels populated
	book.updateAskMin()
	if book.askMin != MAX_PRICE_LEVELS {
		t.Errorf("expected askMin MAX_PRICE_LEVELS for empty book, got %d", book.askMin)
	}
}

func TestUpdateAskMin_SinglePriceLevel(t *testing.T) {
	book := &OrderBook{
		askMin: 5,
	}
	book.askLevels[5] = makePriceLevel(2)

	// Should stay at 5
	book.updateAskMin()
	if book.askMin != 5 {
		t.Errorf("expected askMin 5, got %d", book.askMin)
	}
}

func TestUpdateAskMin_MultipleLevels(t *testing.T) {
	book := &OrderBook{
		askMin: 3,
	}
	book.askLevels[3] = makePriceLevel(1)
	book.askLevels[4] = makePriceLevel(2)
	book.askLevels[6] = makePriceLevel(3)

	// Clear 3, should move to 4
	book.askLevels[3] = PriceLevel{}
	book.updateAskMin()
	if book.askMin != 4 {
		t.Errorf("expected askMin 4, got %d", book.askMin)
	}

	// Clear 4, should move to 6
	book.askLevels[4] = PriceLevel{}
	book.updateAskMin()
	if book.askMin != 6 {
		t.Errorf("expected askMin 6, got %d", book.askMin)
	}

	// Clear 6, should reset to MAX_PRICE_LEVELS
	book.askLevels[6] = PriceLevel{}
	book.updateAskMin()
	if book.askMin != MAX_PRICE_LEVELS {
		t.Errorf("expected askMin MAX_PRICE_LEVELS, got %d", book.askMin)
	}
}

func TestUpdateAskMin_Exhaustive(t *testing.T) {
	book := &OrderBook{}

	// Single ask at 5
	book.askMin = 5
	book.askLevels[5] = makePriceLevel(2)
	book.updateAskMin()
	if book.askMin != 5 {
		t.Errorf("expected askMin 5, got %d", book.askMin)
	}

	// Multiple levels: 5, 7, 9
	book.askLevels[7] = makePriceLevel(1)
	book.askLevels[9] = makePriceLevel(3)

	// Clear 5 -> should move to 7
	book.askLevels[5] = PriceLevel{}
	book.updateAskMin()
	if book.askMin != 7 {
		t.Errorf("expected askMin 7, got %d", book.askMin)
	}

	// Clear 7 -> should move to 9
	book.askLevels[7] = PriceLevel{}
	book.updateAskMin()
	if book.askMin != 9 {
		t.Errorf("expected askMin 9, got %d", book.askMin)
	}

	// Clear 9 -> empty book
	book.askLevels[9] = PriceLevel{}
	book.updateAskMin()
	if book.askMin != MAX_PRICE_LEVELS {
		t.Errorf("expected askMin MAX_PRICE_LEVELS for empty book, got %d", book.askMin)
	}

	// Edge case: ask at MAX_PRICE_LEVELS-1
	lastPrice := MAX_PRICE_LEVELS - 1
	book.askLevels[lastPrice] = makePriceLevel(1)
	book.askMin = Price(lastPrice)
	book.updateAskMin()
	if book.askMin != Price(lastPrice) {
		t.Errorf("expected askMin %d, got %d", lastPrice, book.askMin)
	}
}
