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
	Type           EventType
	OrderID        OrderID
	Price          Price
	Size           Size
	Trader         TraderID
	Symbol         Symbol
	Side           Side
	CounterOrderID OrderID // For executions (counterparty OrderID)
}

// Input command received by matching engine (related to exchange Order struct)
type InputCommand struct {
	Type    EventType
	Symbol  Symbol
	Side    Side
	Price   Price
	Size    Size
	Trader  TraderID
	OrderID OrderID // To allow cancels, not for providing a custom OrderID
}

// StartInputDistributor distributes input commands to the matching engine
func (e *MatchingEngine) StartInputDistributor() {
	buf := make([]InputCommand, DISTRIBUTOR_BUFFER) // Pre-allocated buffer
	for {
		n := e.inputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			ev := &buf[i]
			switch ev.Type {
			case ORDER_EVENT: // New order command
				e.Limit(ev.Symbol, ev.Side, ev.Price, ev.Size, ev.Trader)
			case CANCEL_EVENT: // New cancel command
				e.Cancel(ev.OrderID)
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
