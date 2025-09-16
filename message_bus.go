package main

// Constant defining the size of the distribution buffer
const (
	DISTRIBUTOR_BUFFER = 1 << 10 // 1024 events size
)

// Matching engine event types
type EventType uint8

const (
	INVALID_EVENT   EventType = iota // Invalid event (in default 'zero' position)
	ORDER_EVENT                      // Order creation
	CANCEL_EVENT                     // Order cancellation
	EXECUTION_EVENT                  // Trade execution
	REJECT_EVENT                     // Order rejection
)

// Output event sent by matching engine to report something (eg. Order, execution)
type OutputEvent struct {
	orderID        OrderID
	price          Price
	size           Size
	counterOrderID OrderID // For executions (counterparty OrderID)
	trader         TraderID
	symbol         Symbol
	eventType      EventType
	side           Side
}

// Input command received by matching engine (related to exchange Order struct)
type InputCommand struct {
	price     Price
	size      Size
	orderID   OrderID // To allow cancels, not for providing a custom OrderID
	symbol    Symbol
	trader    TraderID
	eventType EventType
	side      Side
}

// StartInputDistributor distributes input commands to the matching engine
func (e *MatchingEngine) StartInputDistributor() {
	buf := make([]InputCommand, DISTRIBUTOR_BUFFER) // Pre-allocated buffer
	for {
		n := e.inputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			ev := &buf[i]
			switch ev.eventType {
			case ORDER_EVENT: // New order command
				e.Limit(ev.symbol, ev.side, ev.price, ev.size, ev.trader)
			case CANCEL_EVENT: // New cancel command
				e.Cancel(ev.orderID)
			}
		}
	}
}

// StartOutputDistributor distributes output events from the matching engine
func (e *MatchingEngine) StartOutputDistributor(callbackFunc func(OutputEvent)) {
	buf := make([]OutputEvent, DISTRIBUTOR_BUFFER) // Pre-allocated buffer
	for {
		n := e.outputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			callbackFunc(buf[i]) // Call callbackFunc for each output event
		}
	}
}
