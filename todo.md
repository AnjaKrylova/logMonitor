# logMonitor — TODO

Design context and decisions: [overview.md](overview.md).
Every step is TDD: red → green → refactor (see overview).

## Status

**Current:** Exercise 4 — Repository (in progress).
**Next step:** first filter row (red) in `TestMemoryLogRepository_Find`. Note: `slices.Clone(nil)` is `nil` — decide what "no match" returns.

---

## Exercise 1 — Tiny Go service ✅

- [x] HTTP server with timeouts; `GET /health`, `GET /version` returning JSON
- [x] `Content-Type: application/json`; handle startup errors

## Exercise 2 — Log entry & typed log levels ✅

- [x] `LogLevel` type + constants, `LogEntry` struct, `IsError()`
- [x] `ParseLogLevel(s) (LogLevel, error)` — `LevelUnknown` + error for unknown input
- [x] Table tests in `log_test.go` (case, whitespace, empty, invalid)

## Exercise 3 — Real-world log parsing ✅

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
- [x] `ParseAppLog` → `Fields`: `flatten` helper (12 rows) + second decode into `map[string]any`, `mappedKeys` deleted, no extras → `nil` (rules: Decisions in overview)

Optional (can be picked up any time):

- [x] Guard row in `TestParseAppLog` with two broken fields (`{"app":{"name":123},"level":"warn","timestamp":"2025-09-08"}` → envelope time)
- [ ] Per-field checks instead of `reflect.DeepEqual` in `TestParseAppLog` (`%q`, `Timestamp.Equal`)
- [ ] `ParseLog`: convert to UTC once (pick entry, convert, return) instead of in three places
- [ ] `TestParseLog`: row with a non-UTC `receivedAt` (e.g. `receivedAt.In(time.FixedZone("CET", 3600))`) to prove the fallback path converts too

---

## Later exercises

Detail is added when an exercise starts.

- [ ] **4. Repository** — `LogRepository` interface (`Save`, `Find`) + `MemoryLogRepository`; filter by from/to/app. _Interfaces at meaningful boundaries, slices._
  - [x] `LogRepository` + `LogFilter` in `internal/storage/repository.go` (rules: Decisions in overview)
  - [x] `MemoryLogRepository`: save + `Find` with an empty filter
  - [x] `Find` returns a copy (`slices.Clone`), proven by `TestMemoryLogRepository_FindReturnsCopy`
  - [ ] Filter rows: `From` inclusive, `To` exclusive, `App`, combinations, no match → empty
- [ ] **5. Goroutines & channels** — fake producer → channel → consumer → repository; 1,000 logs. _`go`, `chan`, `close`, `range` over a channel._
- [ ] **6. Worker pool** — `Processor{workers}` with `Start(ctx)`; compare 1/2/4/8 workers on 100k logs. _`sync.WaitGroup`, backpressure._
- [ ] **7. Context** — every important operation takes `ctx`; timeouts and cancellation. _`context.WithTimeout`, `defer cancel()`._
- [ ] **8. Ingestion API** — `POST /ingest`: decode, validate, create `LogEntry`, send to a channel. _JSON decoding, validation, handler design._
- [ ] **9. Asynchronous ingestion** — `Ingestor` with a queue; decide what happens when the queue is full (block / reject / drop). _Buffered channels, design trade-offs._
- [ ] **10. Fake Scalingo source** — `LogSource` interface + `FakeLogSource` producing realistic lines. _Interfaces, generators._
- [ ] **11. Multiple applications** — `ApplicationRegistry`, 70 concurrent streams into one channel. _Concurrent producers, lifecycle._
- [ ] **12. Query model** — `LogQuery` (from, to, app, level, error, limit, cursor); `GET /logs`. _Query parsing separate from storage._
- [ ] **13. Aggregation** — hourly buckets, including zero-count hours. _Time bucketing, maps._
- [ ] **14. Statistics APIs** — `GET /stats/apps`, `/stats/errors`, `/stats/hourly`.
- [ ] **15. Redis** — `RedisLogRepository` behind the same interface. _External dependencies, integration tests._
- [ ] **16. Scalingo integration** — `ScalingoLogSource`; investigate drains vs streaming first. App name comes from the source.
- [ ] **17. Retry & backoff** — exponential backoff with a max delay. _`time.NewTimer`, `select` with `ctx.Done()`._
- [ ] **18. Graceful shutdown** — SIGTERM → stop intake → drain queue → close storage. _`signal.NotifyContext`._
- [ ] **19. Time handling** — define "today" (Europe/Copenhagen), store UTC; test midnight, DST, month/year boundaries.
- [ ] **20. Testing review** — fill gaps; `httptest`, `-race`, coverage, benchmarks, integration tests.
