package main

import (
	"fmt"
	"sync/atomic"
	"time"
)

var rng uint64 = 1755956219406641000 // Fixed seed for reproducibility

// Fast xorshift PRNG - much faster than crypto/rand for benchmarking
func fastRand() uint32 {
	rng ^= rng << 13
	rng ^= rng >> 7
	rng ^= rng << 17
	return uint32(rng)
}

func main() {
	engine := NewEngine()

	// Track total inputs / outputs to ensure they broadly match
	var totalInputs uint64
	var totalOutputs uint64

	// Track the recent OrderIDs for generating valid CANCELs
	var recentIDs [DISTRIBUTOR_BUFFER]OrderID
	var recentCount int

	// Start input / output distributors
	go engine.StartInputDistributor()
	go engine.StartOutputDistributor(func(ev OutputEvent) {
		atomic.AddUint64(&totalOutputs, 1) // Increment to demonstrate messages received back

		// Keep recent OrderIDs updated on order events
		if ev.Type == ORDER_EVENT {
			recentIDs[recentCount%DISTRIBUTOR_BUFFER] = ev.OrderID
			recentCount++
		}
	})

	const N = 70_000_000
	start := time.Now()

	for i := 0; i < N; i++ {
		var cmd InputCommand

		// 10% probability of cancel, only if we have recent orders
		if fastRand()%10 == 0 && recentCount > 0 {
			idx := fastRand() % uint32(min(recentCount, DISTRIBUTOR_BUFFER))
			cmd = InputCommand{
				Type:    CANCEL_EVENT,
				OrderID: recentIDs[idx],
			}
		} else {
			cmd = InputCommand{
				Type:   ORDER_EVENT,
				Symbol: Symbol(fastRand() % MAX_SYMBOLS),
				Trader: TraderID(fastRand()%1000 + 1),
				Price:  Price(100 + fastRand()%200),
				Side:   Side(fastRand() % 2),
				Size:   Size(fastRand()%1000 + 1),
			}
		}

		engine.inputRing.Push(cmd)
		atomic.AddUint64(&totalInputs, 1)
	}

	// Wait until all outputs drained
	for atomic.LoadUint64(&totalOutputs) < totalInputs {
		time.Sleep(10 * time.Microsecond)
	}

	elapsed := time.Since(start)
	nsPerOp := float64(elapsed.Nanoseconds()) / float64(N)
	fmt.Printf("%d orders processed in %v -> %d ns/op\n", N, elapsed, int64(nsPerOp))
	fmt.Printf("%d inputs and %d outputs\n", totalInputs, totalOutputs)
}
