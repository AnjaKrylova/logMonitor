# logMonitor — TODO

Design context and decisions: [overview.md](overview.md).
Every step is TDD: red → green → refactor (see overview).

## Status

**Current:** Exercise 8 — Ingestion API (not started).
**Next step:** design `POST /ingest`: what the request body looks like, validation, and where the handler sends entries.

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

- [x] **4. Repository** ✅ — `LogRepository` interface (`Save`, `Find`) + `MemoryLogRepository`; filter by from/to/app. _Interfaces at meaningful boundaries, slices._
  - [x] `LogRepository` + `LogFilter` in `internal/storage/repository.go` (rules: Decisions in overview)
  - [x] `MemoryLogRepository`: save + `Find` with an empty filter
  - [x] `Find` returns a copy (`slices.Clone`), proven by `TestMemoryLogRepository_FindReturnsCopy`
  - [x] Filter rows written (red): `From` inclusive, `To` exclusive (ns boundaries), `App`, combination, no match / empty repo → `nil`
  - [x] `Find` filtering green — one loop, each check `continue`s on fail, single `append`
  - [x] Refactor: early return removed; checks moved to `LogFilter.matches(e) bool` (repository.go)
  - [x] logs package tidy-up: `levelFromStatus` extracted from `ParseRouterLog`, blank lines between steps
- [x] **5. Goroutines & channels** ✅ — fake producer → channel → consumer → repository; 1,000 logs. _`go`, `chan`, `close`, `range` over a channel._
  - [x] `ingest.Producer(n) <-chan logs.LogEntry` — generator: creates, fills, closes; `TestProducer` (run with `-timeout 5s`)
  - [x] `ingest.Consumer(ch, repo) int` — ranges, saves, `slog.Error` + continue on failure, returns saved count
  - [x] 1,000 logs end to end — `TestConsumer` table: 0, 5, 1_000 entries
- [x] **6. Worker pool** ✅ — `Processor{workers}` with `Start(ctx)`; compare 1/2/4/8 workers on 100k logs. _`sync.WaitGroup`, backpressure._
  - [x] `MemoryLogRepository` concurrency-safe (`sync.Mutex` in `Save` + `Find`), proven by `TestMemoryLogRepository_ConcurrentSave` (10,000 saves)
  - [x] `Processor{NumberOfWorkers, Repository}.Run(ch)` — N `Consumer`s via `wg.Go`, blocks until all done; `TestProcessor_Run` (1/2/4/8 workers)
  - [x] Compare 1/2/4/8 workers on 100k logs — `BenchmarkProcessor_Run`: 1 worker fastest (~60 ms), 8 slowest (~73 ms); all workers queue on the repository mutex and the work per entry is tiny. `ctx` deferred to Exercise 7.
- [x] **7. Context** ✅ — every important operation takes `ctx`; timeouts and cancellation. _`context.WithTimeout`, `defer cancel()`._
  - [x] `Producer(ctx, n)` — `select` on `ctx.Done()` vs send, `defer close(ch)` inside the goroutine; `TestProducer_Cancel` (cancel after 5 of 1,000,000); tests use `t.Context()`
  - [x] Processor / Consumer and ctx — decided: drain; only the producer watches `ctx` (see Decisions)
  - [x] Timeout with `context.WithTimeout` — `TestProcessor_Run_Timeout`: 50 ms deadline on 10M entries, `Run` returns in ~70 ms
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
