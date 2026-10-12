# logMonitor — TODO

Design context and decisions: [overview.md](overview.md).
Every step is TDD: red → green → refactor (see overview).

## Status

**Current:** Exercise 3 — Real-World Log Parsing.
**Next step:** `ParseAppLog` → `Fields` (flatten nested values), then Exercise 4.

---

## Exercise 1 — Tiny Go service ✅

- [x] HTTP server with timeouts; `GET /health`, `GET /version` returning JSON
- [x] `Content-Type: application/json`; handle startup errors

## Exercise 2 — Log entry & typed log levels ✅

- [x] `LogLevel` type + constants, `LogEntry` struct, `IsError()`
- [x] `ParseLogLevel(s) (LogLevel, error)` — `LevelUnknown` + error for unknown input
- [x] Table tests in `log_test.go` (case, whitespace, empty, invalid)

## Exercise 3 — Real-world log parsing

Done:

- [x] `parseLogfmt` — lenient, never fails (16 rows)
- [x] `parseTimestamp` + `isTimeZone` (21 rows)
- [x] `ParseEnvelope` — timestamp first, optional `[container]`, `Raw` always set
- [x] `ParseAppLog` — never fails; plain-text fallback only on `*json.SyntaxError`; payload timestamp `== 0` → envelope time
- [x] `TestParseAppLog` refactored: rows take only a payload
- [x] `ParseRouterLog` — logfmt pairs into `Fields`; level from `status` via `strconv.Atoi` (≥500 Error, ≥400 Warn, else Info; missing or non-numeric → Unknown) (7 rows)
- [x] `LogEntry.Fields` changed to `map[string]string`
- [x] Split `parser.go` into `envelope.go`, `applog.go`, `routerlog.go`, `parser.go`
- [x] `ParseLog(rawLine, receivedAt) LogEntry` — dispatches router/app; no timestamp → `Raw`, `LevelUnknown`, `receivedAt`; every entry leaves in UTC (4 rows)

Next:

- [ ] **TDD `ParseAppLog` → `Fields`** — `data.error.message`, `data.error.stack`, `origin`, `params` are decoded but dropped; put them into `Fields`, flattening nested values into string keys (`"origin.file"`, `"origin.line": "158"`; requirement: keep custom structured fields). Not urgent: `Raw` keeps them, nothing queries `Fields` before Exercise 12

Optional:

- [ ] Guard row in `TestParseAppLog` with two broken fields (`{"app":{"name":123},"level":"warn","timestamp":"2025-09-08"}` → envelope time)
- [ ] Per-field checks instead of `reflect.DeepEqual` in `TestParseAppLog` (`%q`, `Timestamp.Equal`, decide `nil` vs empty `Fields`)
- [ ] `ParseLog`: convert to UTC once (pick entry, convert, return) instead of in three places
- [ ] `TestParseLog`: row with a non-UTC `receivedAt` (e.g. `receivedAt.In(time.FixedZone("CET", 3600))`) to prove the fallback path converts too

---

## Later exercises

Detail is added when an exercise starts.

- [ ] **4. Repository** — `LogRepository` interface (`Save`, `Find`) + `MemoryLogRepository`; filter by from/to/app. *Interfaces at meaningful boundaries, slices.*
- [ ] **5. Goroutines & channels** — fake producer → channel → consumer → repository; 1,000 logs. *`go`, `chan`, `close`, `range` over a channel.*
- [ ] **6. Worker pool** — `Processor{workers}` with `Start(ctx)`; compare 1/2/4/8 workers on 100k logs. *`sync.WaitGroup`, backpressure.*
- [ ] **7. Context** — every important operation takes `ctx`; timeouts and cancellation. *`context.WithTimeout`, `defer cancel()`.*
- [ ] **8. Ingestion API** — `POST /ingest`: decode, validate, create `LogEntry`, send to a channel. *JSON decoding, validation, handler design.*
- [ ] **9. Asynchronous ingestion** — `Ingestor` with a queue; decide what happens when the queue is full (block / reject / drop). *Buffered channels, design trade-offs.*
- [ ] **10. Fake Scalingo source** — `LogSource` interface + `FakeLogSource` producing realistic lines. *Interfaces, generators.*
- [ ] **11. Multiple applications** — `ApplicationRegistry`, 70 concurrent streams into one channel. *Concurrent producers, lifecycle.*
- [ ] **12. Query model** — `LogQuery` (from, to, app, level, error, limit, cursor); `GET /logs`. *Query parsing separate from storage.*
- [ ] **13. Aggregation** — hourly buckets, including zero-count hours. *Time bucketing, maps.*
- [ ] **14. Statistics APIs** — `GET /stats/apps`, `/stats/errors`, `/stats/hourly`.
- [ ] **15. Redis** — `RedisLogRepository` behind the same interface. *External dependencies, integration tests.*
- [ ] **16. Scalingo integration** — `ScalingoLogSource`; investigate drains vs streaming first. App name comes from the source.
- [ ] **17. Retry & backoff** — exponential backoff with a max delay. *`time.NewTimer`, `select` with `ctx.Done()`.*
- [ ] **18. Graceful shutdown** — SIGTERM → stop intake → drain queue → close storage. *`signal.NotifyContext`.*
- [ ] **19. Time handling** — define "today" (Europe/Copenhagen), store UTC; test midnight, DST, month/year boundaries.
- [ ] **20. Testing review** — fill gaps; `httptest`, `-race`, coverage, benchmarks, integration tests.
