# femto_go
Prototype HFT exchange: Multi-symbol limit order book in Go. >10M orders/second

## Features:
- Multi-symbol, price-time priority, limit order book matching engine
- High performance, in-memory approach using input and output ring buffers
- Low latency, ~70ns per order (Apple M1)
- ~500 SLOC. Zero dependencies

## Usage:
`go run .`

> [!WARNING]
> Use in a production environment is strongly discouraged, without much more thorough testing and performance tuning
