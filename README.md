# netmon

A layered network health monitor in Go. For each target it checks DNS, then TCP,
then HTTP, and stops at the first failing layer, so a failure is reported with
*where* it happened, not just "down".

## Run

    go run . -config targets.json

## Status

- [x] Config loading (targets.json)
- [x] DNS check (lookup latency)
- [x] TCP check (handshake latency)
- [x] Concurrent probing, one goroutine per target
- [ ] Day 1: HTTP/TLS check (status code, latency, cert expiry)
- [ ] Day 1: Ping check (loss %, avg RTT; informational only)
- [ ] Day 2: Run on an interval (time.Ticker)
- [ ] Day 2: Store results in SQLite; uptime % and p95 latency per target
- [ ] Day 2: Alert on UP -> DOWN / DOWN -> UP transitions
- [ ] Day 2: Sample output + design notes in this README
