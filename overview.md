# logMonitor — Overview

Progress and next steps: [todo.md](todo.md).

## Goal

A Go backend that collects logs from about 70 Scalingo applications, stores
them, and exposes an API to query and aggregate them. A frontend comes later.

It is also a learning project: Go concepts are learned by building a real
service, with TypeScript backend experience as the starting point.

## Requirements

The service should eventually:

- ingest continuous logs from ~70 Scalingo apps, surviving connection failures
- parse Scalingo router logs and custom JSON app logs
- store raw events and keep custom structured fields
- query by time range; filter and group by app and by error
- count errors per hour, including zero-count hours
- drill down from an aggregate to the individual events
- shut down gracefully

## Architecture

```text
Scalingo (70 apps) ── log drain / stream ──> Go ingestion
                                                 │
                                          buffered channel
                                                 │
                                       worker  worker  worker
                                                 │
                                       parse + normalize
                                          ┌──────┴──────┐
                                     raw events     aggregations
                                          └──────┬──────┘
                                          storage (memory → Redis)
                                          ┌──────┴──────┐
                                        /logs         /stats ──> frontend
```

What exists today (`internal/ingest`):

```text
Producer(n) ──> channel ──> Processor.Run ──> N × Consumer ──> LogRepository.Save
 makes entries   carries     starts N workers    each: read,        stores them
 (fake source)   entries     and waits for all   save, count        (mutex-safe)
```

- **Producer**: where entries come from (fake now; `LogSource` in Exercise 10).
- **Consumer**: one worker, reads the channel and saves each entry.
- **Processor**: the manager, runs N consumers on the same channel.

Principles:

- **Depend on interfaces at the edges.** `LogSource` (fake → Scalingo) and
  `LogRepository` (memory → Redis) can be swapped without touching the rest.
- **Raw events are the source of truth; aggregations are derived data.**
  Raw events serve drill-down and debugging; aggregations serve charts.
- **Memory first, Redis later**, then re-evaluate: the queries are
  analytical (time ranges, grouping, retention), so Redis may not be the
  final answer.
- **Aggregate on a stable error code, not the message.** `data.error.code`
  (e.g. `FETCH_CLIENT_ERROR_REQUEST_TIMEOUT`) groups errors; "user 123 not
  found" and "user 456 not found" must not be two categories.
- **Let packages evolve.** Start in `internal/logs`; split into packages
  (ingestion, storage, http, …) only when they become useful.

## Domain model

`LogEntry` in `internal/logs/log.go` is the source of truth. In short:
timestamp, app, container, level, message, error code, custom `Fields`
(string keys and values), and
`Raw` — the original line, set on **every** entry.

## Real log lines

Router (logfmt; level derived from `status`):

```text
2026-06-24 15:02:19.446989072 +0200 CEST [router] method=GET path="/password" host=id.hedia.com status=200 duration=0.030s bytes=5332
```

App (JSON; level, message, error code and app name inside the payload):

```text
2025-09-08 13:12:58.867176204 +0200 CEST [web-1] {"app":{"name":"demo-app","version":"3.4.2"},"data":{"error":{"code":"FETCH_CLIENT_ERROR_REQUEST_TIMEOUT",...}},"level":"error","message":"Error processing request","timestamp":1757329978852}
```

## Parsing pipeline

1. **`ParseEnvelope(line)`** — timestamp (via `parseTimestamp`), optional
   `[container]`, payload, `Raw`.
2. **Stage 2**, chosen by container:
   - `ParseRouterLog(env)` for `router` — logfmt via `parseLogfmt`
   - `ParseAppLog(env)` for everything else — JSON
3. **`ParseLog(rawLine)`** — the dispatcher that ties it together.

Policy: **never lose a log.** Stage-2 parsers never fail; they parse what
they can and fall back for the rest. Low-level helpers (`ParseLogLevel`,
`parseTimestamp`) report errors; the higher level decides the fallback.

## Scalingo log drains (findings, 2026-10-22)

From [the log drain docs](https://doc.scalingo.com/platform/app/log-drain):
syslog over UDP / TCP / TCP+TLS is supported. A target that fails to accept
10 consecutive lines is **quarantined for 5 minutes** (no lines sent), and
the quarantine grows (10, 15, 20 min…) while it keeps failing. Retries,
buffering or replay are **not documented**: assume lines rejected or sent
during quarantine are lost for us. *Consequence for Exercise 9:* rejecting
under load is risky; the ingestion side should accept fast and absorb
bursts. Ask Scalingo support for the exact rules in Exercise 16.

## Decisions

Each entry: what was decided — why — the alternative not chosen.

- **TDD for every step.** Tests are the spec; when a decision changes, the
  `want` values change first. — Tests written at the end were being skipped.
- **`Raw` is captured on the envelope and always set**, even when parsing
  fails; each stage-2 parser copies it. — Every parser returns a complete,
  testable entry. *Not chosen:* setting `Raw` only in `ParseLog`.
- **The envelope parses the timestamp first.** Date, clock and offset are
  parsed; the zone word is skipped only if it is 3–4 uppercase letters. —
  Finding `[` first broke on brackets inside payloads. *Accepted trade-off:*
  a short uppercase word after the offset (e.g. `WARN`) is taken as a zone.
  *Changed 2026-10-26:* real lines glue the container to the zone
  (`CEST[web-1]`, no space); the zone check splits the first word at `[`
  and keeps `[container]` in `rest`. Found by parsing real drain lines.
- **Every entry leaves `ParseLog` in UTC** — one choke point converts the
  timestamp on every path; stage parsers may return local times.
  `TestParseLog` checks `Location() == time.UTC` in every row. — Storage and
  hourly buckets need one location, `reflect.DeepEqual` compares the location,
  and a rule enforced at the boundary cannot be forgotten at call sites.
  *Not chosen:* `.UTC()` in each parser (`parseTimestamp`, `ParseAppLog`).
- **`[container]` is optional.** A `[` without `]` is not handled. — Not
  seen in real Scalingo lines; `Raw` keeps the line.
- **No timestamp → envelope with only `Raw`, plus an error.** Nothing in such
  a line is trusted. — Real Scalingo lines always have a timestamp.
- **No line is dropped.** A line without a timestamp is stored unstructured:
  `Raw`, `LevelUnknown`, fallback timestamp.
- **The fallback timestamp is the receive time, passed by the caller:**
  `ParseLog(rawLine string, receivedAt time.Time)`. — Deterministic tests;
  re-parsing old lines from `Raw` keeps their original time. *Not chosen:*
  `time.Now()` inside the parser; leaving it at zero.
- **`ParseLog` returns `(LogEntry, error)` (changed 2026-10-24; was: no
  error).** The entry is always usable (`Raw` set, fallback timestamp) even
  when `err != nil`; the doc comment says so. The error is set only when the
  envelope fails (no timestamp), not for plain-text app lines or router lines
  without status. — Callers (handler) can `slog.Warn` the reason instead of
  guessing from `LevelUnknown`; consistent with `ParseEnvelope` and
  `ParseLogLevel`. *Not chosen:* keep swallowing the error; a `ParseError`
  field on `LogEntry` (possible later, to query failed lines).
- **`ParseLogLevel` returns `LevelUnknown` together with an error.** — Callers
  get a usable value and still learn what was wrong.
- **`LevelUnknown`** for missing or unrecognised levels. — Keeps malformed
  lines queryable without distorting error/warning counts.
- **App payload: only `*json.SyntaxError` falls back to plain text**
  (`Message` = payload, envelope time). A wrong-type field
  (`*json.UnmarshalTypeError`) keeps every field that decoded.
- **Payload timestamp `== 0` → envelope time.** — A zero int would become
  1970. *Not chosen:* `UnmarshalTypeError.Field == "timestamp"`, because
  `json.Unmarshal` reports only the earliest type error.
- **A bare JSON value (e.g. `42`) takes the JSON path**: envelope time,
  `LevelUnknown`, empty message. *Not chosen:* treating it as plain text.
- **`parseLogfmt` is lenient**: empty values allowed, pairs without a key
  skipped, unclosed quote runs to the end, last duplicate key wins. *Accepted
  trade-off:* a bad quote can swallow following pairs; `Raw` keeps the line.
- **App name comes from the ingestion source**, with the payload's
  `app.name` as a fallback. — The envelope does not contain the app.
- **`LogEntry.Fields` is `map[string]string`.** — Typed and simple; logfmt
  values are strings anyway; no `any` traps (`200` vs `"200"` look identical
  when printed). *Consequence:* nested app values (`origin`, `params`,
  `stack`) must be flattened into string keys (`"origin.file"`,
  `"origin.line": "158"`), and numbers lose their type — numeric filters
  must convert back. *Not chosen:* `map[string]any`.
- **Router entries leave `Message` and `App` empty**; everything is in
  `Fields`. — Simplest; `host` is a hostname, not the app. *Possible later:*
  a summary message such as `"GET /password 200"` built from `Fields`.
- **App `Fields` = every payload key not already mapped to a `LogEntry`
  field** (`app.name`, `level`, `message`, `timestamp`, `data.error.code`
  are skipped), flattened with dotted keys; array items as `params.0`;
  empty arrays/objects and `null` add no key; no extras → `Fields` is `nil`.
  — Keeps unknown custom fields; `null` is "nothing here" like `[]`.
  *Not chosen:* a fixed list of known keys; `null` → `"k": ""` (fake value,
  indistinguishable from a real empty string; `Raw` keeps the original).
- **Storage lives in `internal/storage`**, interface next to its
  implementations (memory, later Redis). — `logs` stays about entries and
  parsing; `storage` imports `logs`, never the reverse. *Not chosen:*
  interface on the consumer side (no consumer exists yet).
- **`LogFilter` uses `time.Time`, half-open `[From, To)`; zero-value fields
  mean "no limit".** — Typed comparison in every implementation; adjacent
  windows (hourly buckets) never double-count. Parsing text → time belongs to
  the query layer. *Not chosen:* string times; inclusive `To`.
- **`Save`/`Find` return `error`** even though memory never fails. — Redis
  behind the same interface can.
- **`Find` with no match returns `nil`**, not `[]LogEntry{}`. — Simplest and
  idiomatic: a nil slice behaves like an empty one (`len`, `range`, `append`),
  and filtering with `append` / `slices.Clone(nil)` produce it naturally.
  *Consequence:* JSON encodes it as `null`; the `GET /logs` handler
  (Exercise 12) turns it into `[]`. *Not chosen:* always non-nil.
- **`Find` returns a new slice** (built with `append`), never the internal
  one. — Callers can't change stored entries through the result.
- **Moving entries into the system lives in `internal/ingest`** (producer,
  consumer; later the processor and ingestor). — Keeps `logs` (parsing) and
  `storage` (saving) unaware of channels. *Not chosen:* `processdata` (vague).
- **`MemoryLogRepository` is safe for concurrent use: a `sync.Mutex` inside,
  locked in both `Save` and `Find`.** — Workers save in parallel, and HTTP
  handlers (Exercise 12) will `Find` meanwhile. Mutex protects state;
  channels move work. *Not chosen:* a single owner goroutine with
  request/reply channels (≈3× the code for the same result).
- **Cancellation drains: only the producer watches `ctx`.** On cancel it
  stops sending and closes the channel; consumers finish what is in flight
  and `Run` returns. — No log already in the pipeline is lost; matches the
  graceful shutdown plan (Exercise 18). *Not chosen:* consumers also stop on
  `ctx` (faster, but drops entries). Consumers get `ctx` later only to pass
  into `Save` (Redis, Exercise 15).
- **`POST /ingest` takes a raw Scalingo-format line** (`{"line": "..."}`)
  and the handler calls `ParseLog`. — The same parsing then serves a future
  syslog listener (Scalingo drains are syslog, Exercise 16); only transport
  differs. *Not chosen:* client sends structured fields (duplicates parsing
  on the client, diverges from what Scalingo sends).
- **HTTP handlers live in `internal/api`.** The ingest handler is a
  *producer*: a struct holding a send-only `chan<- logs.LogEntry`; `main.go`
  makes the channel and gives the send side to the handler, the receive side
  to `Processor.Run`. — HTTP concerns (decoding, status codes) stay out of
  `ingest`.
- **Ingestion queue: buffered channel of 10,000; when full, log an error and
  then block until there is room.** — Absorbs bursts so Scalingo gets a fast
  202 (rejects/timeouts risk quarantine, see findings above); nothing is
  dropped; the error log shows in production how often the queue fills, to
  decide on scaling. *Not chosen:* drop (loses logs), reject with 503
  (Scalingo doesn't document retries), block silently (no visibility).
- **Logging with `log/slog`, at the edges only** (handlers, workers,
  sources, `main`); library packages like `logs` and `storage` return
  information/errors and never log. Levels: **Error** = something in our
  service is broken (save failed, queue full, Redis down); **Warn** =
  unusual but handled (bad client request, line without timestamp, retry);
  **Info** = normal lifecycle (start, stop, shutdown steps). — Callers have
  the context to decide; libraries stay testable and quiet.
- **`LogSource` yields raw lines: `Stream(ctx) <-chan string`** (generator:
  closes when done or cancelled). A small parse step turns lines into
  entries with `ParseLog` and sends them to the same entries channel the
  HTTP handler uses. — Scalingo sends raw lines; parsing stays in one place.
  *Not chosen:* sources emit `LogEntry` (each source would parse itself).
- **Push model: all 70 Scalingo apps drain to one endpoint** (already set
  up in production). Fan-in happens at the endpoint (one goroutine per
  request/connection, all into `entries`), so Exercise 11 became a load
  simulation plus "where does the app name come from". *Deferred:* an
  `ApplicationRegistry` for pull streams (optional, only if ever needed).
- **Filter rules live in `LogFilter.matches(e)`**, next to the struct. — One
  place defines what a filter means; `Find` just loops and keeps matches.

Possible later: record the malformed field in `Fields["parse_error"]` to
monitor apps that send broken logs.

## Development philosophy

For each step:

1. **Red** — write a failing test (table-driven where it fits); watch it fail
   for the expected reason.
2. **Green** — the smallest change that passes.
3. **Refactor** — clean up while green.
4. Break it on purpose and see which test catches it.

The user writes all implementation code; Claude gives hints, reviews, and
writes tests or docs on request. The goal is not only a working service but
an intuition for how Go works.
