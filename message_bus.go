package main

const (
	DISTRIBUTOR_BUFFER = 1 << 10 // 1024 event size
)

// Exchange engine event types
type EventType uint8

const (
	INVALID_EVENT   EventType = iota // Invalid event (in default position)
	ORDER_EVENT                      // Order creation
	CANCEL_EVENT                     // Order cancellation
	EXECUTION_EVENT                  // Trade execution
	REJECT_EVENT                     // Order rejection
)

// Output event sent by exchange engine
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

// Input command received by exchange engine (related to exchange Order struct)
type InputCommand struct {
	Type    EventType
	Symbol  Symbol
	Side    Side
	Price   Price
	Size    Size
	Trader  TraderID
	OrderID OrderID
}

// Distributes commands to engine
func (e *Engine) StartInputDistributor() {
	buf := make([]InputCommand, DISTRIBUTOR_BUFFER)
	for {
		n := e.inputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			ev := &buf[i]
			switch ev.Type {
			case ORDER_EVENT:
				e.Limit(ev.Symbol, ev.Side, ev.Price, ev.Size, ev.Trader)
			case CANCEL_EVENT:
				e.Cancel(ev.OrderID)
			}
		}
	}
}

// Distributes commands from engine
func (e *Engine) StartOutputDistributor(callbackFunc func(OutputEvent)) {
	buf := make([]OutputEvent, DISTRIBUTOR_BUFFER)
	for {
		n := e.outputRing.Read(buf)
		for i := 0; uint32(i) < n; i++ {
			callbackFunc(buf[i])
		}
	}
}
