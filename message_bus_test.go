package main

import (
	"testing"
	"time"
)

// Helper to read one or more OutputEvent(s) from the engine.outputRing with timeout.
// Returns the slice of read events or nil on timeout.
func readOutputEvents(e *MatchingEngine, max int, timeout time.Duration) []OutputEvent {
	resCh := make(chan []OutputEvent, 1)

	go func() {
		buf := make([]OutputEvent, max)
		n := e.outputRing.Read(buf)
		events := make([]OutputEvent, n)
		for i := 0; i < int(n); i++ {
			events[i] = buf[i]
		}
		resCh <- events
	}()

	select {
	case evs := <-resCh:
		return evs
	case <-time.After(timeout):
		return nil
	}
}

func TestStartInputDistributor_OrderProducesOrderEvent(t *testing.T) {
	e := NewMatchingEngine()

	go e.StartInputDistributor()

	// Push an InputCommand into inputRing.
	cmd := InputCommand{
		eventType: ORDER_EVENT,
		symbol:    1,
		side:      Bid,
		price:     10,
		size:      5,
		trader:    7,
	}
	e.inputRing.Push(cmd)

	// Read from output ring - Limit should have pushed an ORDER_EVENT.
	events := readOutputEvents(e, 4, 200*time.Millisecond)
	if events == nil {
		t.Fatalf("timed out waiting for output events")
	}

	found := false
	for _, ev := range events {
		if ev.eventType == ORDER_EVENT {
			found = true
			// basic sanity checks
			if ev.price != 10 || ev.size != 5 || ev.trader != 7 || ev.symbol != 1 {
				t.Fatalf("order event fields mismatch: got %+v", ev)
			}
		}
	}
	if !found {
		t.Fatalf("expected ORDER_EVENT but none found in %+v", events)
	}
}

func TestStartInputDistributor_CancelProducesCancelEvent(t *testing.T) {
	e := NewMatchingEngine()

	go e.StartInputDistributor()

	// 1) Create an order first by sending an ORDER_EVENT command.
	createCmd := InputCommand{
		eventType: ORDER_EVENT,
		symbol:    2,
		side:      Ask,
		price:     20,
		size:      3,
		trader:    9,
	}
	e.inputRing.Push(createCmd)

	// Read until we find the ORDER_EVENT and capture the OrderID generated.
	var createdOrderID OrderID = 0
	timeout := time.After(200 * time.Millisecond)
loopCreate:
	for {
		select {
		case <-timeout:
			t.Fatalf("timed out waiting for order creation event")
		default:
			events := readOutputEvents(e, 4, 50*time.Millisecond)
			if events == nil {
				// try again until overall timeout
				continue
			}
			for _, ev := range events {
				if ev.eventType == ORDER_EVENT {
					createdOrderID = ev.orderID
					break loopCreate
				}
			}
		}
	}

	if createdOrderID == 0 {
		t.Fatalf("did not receive ORDER_EVENT with a valid OrderID")
	}

	// 2) Now send a CANCEL_EVENT for that OrderID.
	cancelCmd := InputCommand{
		eventType: CANCEL_EVENT,
		orderID:   createdOrderID,
	}
	e.inputRing.Push(cancelCmd)

	// Verify we receive a CANCEL_EVENT in outputRing.
	foundCancel := false
	timeout2 := time.After(200 * time.Millisecond)
	for !foundCancel {
		select {
		case <-timeout2:
			t.Fatalf("timed out waiting for CANCEL_EVENT")
		default:
			events := readOutputEvents(e, 4, 50*time.Millisecond)
			if events == nil {
				continue
			}
			for _, ev := range events {
				if ev.eventType == CANCEL_EVENT && ev.orderID == createdOrderID {
					foundCancel = true
					break
				}
			}
		}
	}
}

func TestStartOutputDistributor_CallbackInvoked(t *testing.T) {
	e := NewMatchingEngine()

	// Channel to capture callback invocations.
	cbCh := make(chan OutputEvent, 4)

	// Start the output distributor with a callback that forwards to cbCh.
	go e.StartOutputDistributor(func(ev OutputEvent) {
		cbCh <- ev
	})

	// Push an OutputEvent into the engine's output ring.
	out := OutputEvent{
		eventType: REJECT_EVENT,
		orderID:   0,
	}
	e.outputRing.Push(out)

	// Wait for callback to be invoked.
	select {
	case got := <-cbCh:
		if got.eventType != REJECT_EVENT {
			t.Fatalf("callback received wrong event type: %+v", got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("timed out waiting for callback invocation")
	}
}
