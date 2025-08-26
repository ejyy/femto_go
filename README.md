# femto_go
Prototype HFT exchange: Multi-symbol limit order book in Go. ~20M orders/second

## Features:
- Multi-symbol, price-time priority, limit order book matching engine
- High performance, in-memory approach using input and output ring buffers
- Low latency, ~70ns per order (Apple M1)
- ~400 SLOC. Zero dependencies. Includes optional TCP server for client order entry / distribution

## Usage:
`go run .`

> [!WARNING]
> Use in a production environment is strongly discouraged, without much more thorough testing and performance tuning
