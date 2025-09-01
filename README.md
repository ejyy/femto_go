# femto_go
Prototype HFT exchange: Multi-symbol limit order book in Go. ~20M orders/second

## Features:
- Multi-symbol, price-time priority, limit order book matching engine
- High performance, in-memory approach using input and output ring buffers
- Low latency, ~50ns per order (Apple M1)
- ~400 SLOC. Zero dependencies

## Usage:
`go run .`

> [!WARNING]
> Use in a production environment is strongly discouraged, without much more thorough testing and performance tuning

## TODO
- [ ] Consider checking if price level exhausted before updating bidmax
