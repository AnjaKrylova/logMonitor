# logMonitor — TODO

Design context and decisions: [overview.md](overview.md).
Every step is TDD: red → green → refactor (see overview).

## Status

**Current:** Exercise 14 of 37 — cursor. (Exercise 13, `limit`, done 2026-11-09.)
**Next step:** cursor — "next page starts here". First decide what a cursor points at (a timestamp? an ID?) and why offset (`skip`) breaks when new logs arrive while paging.

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
- [x] Per-field checks instead of `reflect.DeepEqual` in `TestParseAppLog` — solved with `cmp.Diff` (go-cmp, 2026-11-08)
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
- [x] **8. Ingestion API** ✅ — `POST /ingest`: decode, validate, create `LogEntry`, send to a channel. _JSON decoding, validation, handler design._
  - [x] `api.Handler{Channel}.ServeHTTP` — decode `{"line"}`, `ParseLog(line, time.Now())`, send, 202; `TestHandler_ServeHTTP` (httptest)
  - [x] Validation → 400 — table in `TestHandler_ServeHTTP` with raw bodies: invalid JSON, non-string, missing, empty `line`; nothing sent on 400
  - [x] Wired into `main.go`: memory repo, channel, `go p.Run(ch)` (2 workers), `mux.Handle("POST /ingest", h)`; tried with `curl.exe` (202 valid, 400 broken)
- [x] **9. Asynchronous ingestion** ✅ — `Ingestor` with a queue; decide what happens when the queue is full (block / reject / drop). _Buffered channels, design trade-offs._
  - [x] Queue full → `slog.Error`, then blocking send (`select` + `default`); `TestHandler_ServeHTTP_QueueFull` uses `testing/synctest` (`synctest.Wait` before making room) so the handler really meets a full queue
  - [x] `main.go`: buffer 10_000
  - [x] `ParseLog` returns `(LogEntry, error)` — entry always usable, error passed through from `ParseEnvelope` (no timestamp, empty line); `TestParseLog` has `wantErr`
  - [x] Logging in the handler: `slog.Warn` on each 400 and on a parse error; a line without timestamp is still sent (202) — table row with `wantRaw`
- [x] **10. Fake Scalingo source** ✅ — `LogSource` interface + `FakeLogSource` producing realistic lines. _Interfaces, generators._
  - [x] `LogSource` + `FakeLogSource{NumLines}.Stream(ctx)`: alternating router (logfmt) + app (JSON) lines in Scalingo format; `TestFakeLogSource_Stream` checks count, no parse error, no `LevelUnknown`
  - [x] `ingest.Parse(in <-chan string, out chan<- logs.LogEntry)` — `ParseLog` each line, `slog.Warn` on error and still send, never closes `out`; `TestParse`
  - [x] Wired into `main.go`: `FakeLogSource{5}.Stream` → `go ingest.Parse(lines, entries)`; `entries` (10_000) shared by Parse, Processor and the HTTP handler
- [ ] **11. Multiple applications (reshaped 2026-10-26)** — all 70 apps push to one endpoint (Scalingo drains), so there are no per-app streams to manage. _Fan-in, load simulation._
  - [x] Bug from real lines: `CEST[web-1]` (no space) lost container/app/level — `parseTimestamp` now cuts a glued zone; test data switched to the real format
  - [ ] Load simulation: fake apps sending lines concurrently to `POST /ingest` (`httptest.NewServer` + real pipeline); one goroutine per container (up to 4 `web-N`, 4 `worker-N`, `router` = 9 per app); rows (start small, grow): **50 apps × 1 web + 1 worker**, **100 apps × 2 web + 2 worker**, **100 apps × 4 web + 4 worker**, **200 apps × 4 web + 4 worker** = future worst case (+ `router` per app → 150 / 500 / 900 / 1,800 senders); knobs: apps, web and worker containers per app, lines per container; check every line saved, measure time, see whether "queue is full" appears
  - [x] App name decided (2026-10-27): `App` only from the app JSON (`app.name`), else empty; `LogEntry.Host` from router `host=`, removed from `Fields` (done 2026-10-28); router lines are kept (useful for 5xx / latency stats) — revisit if the load test shows a performance problem
- [x] **12. Query model — the Logs list screen** ✅ (done 2026-11-08; paging moved to 13 + 14) (design: real UI screenshots, 2026-10-30) — `GET /logs`. _Query parsing separate from storage._ Logging: `slog.Warn` on 400 (bad query params); `slog.Error` + 500 when `Find` fails.
  - [x] `GET /logs` with `from`, `to`, `app` → `LogFilter` → `Find` → JSON (`[]` not `null`); newest first (done 2026-11-07; `parseTimeQueryParam` helper, `Find` sorts newest first with `slices.SortStableFunc`)
  - [x] Container type: `worker-1` → `worker` (edge case `one-off-1234` → `one-off`); filter by it (done 2026-11-07; `LogEntry.ContainerType()` method, `?container_type=`)
  - [x] Message template + `MessageParams` (decided 2026-11-08, see overview): `"User %s not found"` + `["123"]`; the UI fills in the gap
    - [x] `LogEntry.MessageParams []string` (log.go)
    - [x] `appLogPayload.MessageParams` with tag `json:"params"`; `ParseAppLog` copies it — strings only; empty → `nil` like `Fields`
    - [x] Numbers in params: `["a", 3]` → `"a"`, `"3"` (`paramsToStrings`, 2026-11-08). Open: a `null` param is skipped, which shifts later params to the wrong gap — Node prints `"null"`
    - [x] Remove `params.*` from `Fields` (`delete(raw, "params")` before `flatten`)

  Optional (same filter pattern as `app`; good for low-energy days, can be picked up any time):

  - [ ] Filter by level
  - [ ] Filter by message template
  - [ ] Request ID (`request_id`): filter "everything for this request"
  - [ ] Search text (message contains)
From here on, **every exercise is one session** (renumbered 2026-11-08). Review each before starting; split again if one turns out bigger.

_Logs list — paging_
- [x] **13. `limit`** ✅ (done 2026-11-09; `parseIntQueryParam` with a default, range check in the handler, cut in `Find`) — default 50, 1–500, else 400; `LogFilter.Limit` (0 = no limit), `Find` cuts after sorting; handler parses `?limit=`
- [ ] **14. Cursor** — "next page starts here": the response carries a cursor, the client sends it back

_Log detail screen_ — params, data (re-parse `Raw` for nested JSON incl. stack), source, container + type, origin, raw line
- [ ] **15. IDs** — assign `LogEntry.ID` on ingest (unique, e.g. random hex); `Find` returns it
- [ ] **16. `GET /logs/{id}`** — `r.PathValue("id")`, 404 when unknown, the detail response

_Dashboard screen_ — group by app + container type + level + message template. _Time bucketing, maps._
- [ ] **17. Count per group** — a group key struct as a map key → `map[GroupKey]int` for one day
- [ ] **18. Hourly series** — 24 slots per group, `Timestamp.Hour()` → slot; a fixed `[24]int` keeps zero hours
- [ ] **19. Dashboard order** (small) — sort groups by count, biggest first; ties stable

_Statistics API_ — logging same as `/logs` (`Warn` on 400, `Error` on 500)
- [ ] **20. `GET /stats`** — groups with count + hourly series for one day; `?day=`
- [ ] **21. Stats filters** — app, container type, level (reuse `/logs` parsing)

_Redis_ — `RedisLogRepository` behind the same interface. Logging: `slog.Error` when Redis is unreachable; repository returns errors, callers log them
- [ ] **22. Meet Redis** — run it locally; a list, a hash and a sorted set in `redis-cli`; pick a Go client (`go get`), ping from a tiny test
- [ ] **23. Storage shape** (mostly thinking) — how entries are stored; how "newest first", time ranges and `limit` are queried
- [ ] **24. Redis `Save`** — integration test that skips when Redis isn't running
- [ ] **25. Redis `Find`** — run the existing storage test table against Redis too
- [ ] **26. Errors, `ctx`, `main.go`** — wrap with `%w`; `ctx` into `Save`/`Find`; switch `main.go`

_Scalingo integration_ — `ScalingoLogSource`. Logging: `slog.Warn` on malformed syslog frames; `slog.Error` when the listener fails
- [ ] **27. Drain research** (no code) — syslog format, protocol, connections (how many, stay open?), where the app name is; questions for Scalingo support
- [ ] **28. Parse a syslog frame** — table test with real examples → raw line + app name
- [ ] **29. TCP listener** — `net.Listen`, one connection, frames → `entries`
- [ ] **30. Many connections** — a goroutine per connection; reuse the load-test idea
- [ ] **31. End to end** — one real app's drain

_Robustness_
- [ ] **32. Retry & backoff** — exponential backoff with a max delay. _`time.NewTimer`, `select` with `ctx.Done()`._ `slog.Warn` per retry; `slog.Error` when giving up
- [ ] **33. Shutdown: catch the signal, stop intake** — `signal.NotifyContext`, `http.Server.Shutdown`; `slog.Info` per step
- [ ] **34. Shutdown: drain and close** — wait for workers, close storage, a timeout; `slog.Error` on timeout or close failure

_Time and tests_
- [ ] **35. "Today"** — Europe/Copenhagen, stored in UTC; `time.LoadLocation`, day start/end for a given date
- [ ] **36. Time edge cases** (tests) — midnight, DST switch days (23/25 hours), month/year boundaries
- [ ] **37. Testing review** — fill gaps; `-race`, coverage, benchmarks
- [ ] **Later (optional): `ApplicationRegistry` for pull streams** — start/stop several `LogSource` streams (one per app, fan-in to `entries`); `slog.Info` on start/stop, `slog.Error` when a stream ends unexpectedly. Only if a pull source is ever needed.
