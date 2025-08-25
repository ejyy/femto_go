package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

var totalInputs uint64
var totalOutputs uint64
var rng uint64 = uint64(1755956219406641000) // Fixed seed for reproducibility

// Fast xorshift PRNG - much faster than crypto/rand for benchmarking
func fastRand() uint32 {
	rng ^= rng << 13
	rng ^= rng >> 7
	rng ^= rng << 17
	return uint32(rng)
}

func main() {
	engine := NewEngine()

	// server := NewServer(engine) // Setup TCP server

	// Start input / output distributors
	go engine.InputDistributor()
	go engine.OutputDistributor(func(ev OutputEvent) {
		atomic.AddUint64(&totalOutputs, 1) // Increment to demonstrate messages received back
		// server.serverDistributionCallback(ev) // Report events to connected server clients
	})

	// server.Start() // Start TCP server

	const N = 30_000_000
	start := time.Now()

	for i := 0; i < N; i++ {
		if fastRand()%10 == 0 && engine.orderID > 0 {
			engine.inputRing.Push(InputCommand{
				Type:    CANCEL_EVENT,
				OrderID: OrderID(fastRand()%uint32(engine.orderID) + 1),
			})
		} else {
			engine.inputRing.Push(InputCommand{
				Type:   ORDER_EVENT,
				Symbol: Symbol(fastRand() % MAX_SYMBOLS),
				Trader: TraderID(fastRand()%1000 + 1),
				Price:  Price(100 + fastRand()%200),
				Side:   Side(fastRand() % 2),
				Size:   Size(fastRand()%1000 + 1),
			})
		}
		atomic.AddUint64(&totalInputs, 1)
	}

	// Wait until all outputs drained
	for atomic.LoadUint64(&totalOutputs) < totalInputs {
		time.Sleep(1 * time.Nanosecond)
	}

	elapsed := time.Since(start)
	nsPerOp := float64(elapsed.Nanoseconds()) / float64(N)
	fmt.Printf("%d orders processed in %v -> %d ns/op\n", N, elapsed, int64(nsPerOp))
}
