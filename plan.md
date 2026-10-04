# Go Log Service — Learning & Implementation Plan

## 1. Project Overview

Build a backend service in Go that collects logs from approximately 70 Scalingo applications, stores them efficiently, and exposes an API for querying and aggregating those logs.

The frontend will be added later.

The project should also serve as a practical Go learning project. Since the developer already has backend experience with TypeScript, the focus should be on learning Go concepts through progressively more realistic exercises.

---

# 2. Main Requirements

The service should eventually support:

- Ingesting logs from approximately 70 Scalingo applications
- Handling continuous/never-ending log ingestion
- Parsing Scalingo logs and custom application logs
- Storing raw log events
- Querying logs by time range
- Filtering logs by application
- Filtering logs by error
- Grouping logs by application
- Grouping logs by error
- Counting occurrences of errors
- Aggregating errors by hour
- Returning zero-count hours
- Retrieving individual log events behind an aggregate
- Preserving custom structured logging fields
- Supporting future filtering by arbitrary custom fields
- Handling connection failures and retries
- Gracefully shutting down
- Eventually supporting a frontend dashboard

---

# 3. High-Level Architecture

The eventual architecture should look approximately like this:

```text
                         Scalingo
                    70 applications
                           │
                           │ log drain / stream
                           ▼
                  ┌─────────────────┐
                  │  Go ingestion   │
                  │                 │
                  │ HTTP / syslog   │
                  └────────┬────────┘
                           │
                           ▼
                     buffered channel
                           │
                ┌──────────┼──────────┐
                ▼          ▼          ▼
             worker     worker     worker
                │          │          │
                └──────────┼──────────┘
                           ▼
                      normalization
                           │
                 ┌─────────┴─────────┐
                 ▼                   ▼
             raw events          aggregations
                 │                   │
                 └─────────┬─────────┘
                           ▼
                         Redis
                           │
                 ┌─────────┴─────────┐
                 ▼                   ▼
              /logs               /stats
                 │                   │
                 └─────────┬─────────┘
                           ▼
                       Frontend
```

---

# 4. Important Architectural Principle

Keep the application independent from external systems.

In particular:

```text
Go application
    │
    ├── LogSource interface
    │
    └── LogRepository interface
```

The implementation can then be changed without changing the rest of the application.

For example:

```text
LogSource
    ├── FakeLogSource
    └── ScalingoLogSource

LogRepository
    ├── MemoryLogRepository
    └── RedisLogRepository
```

This allows development and testing without immediately depending on Scalingo or Redis.

---

# 5. Raw Events vs Aggregations

The system needs two different types of data.

## Raw events

Individual log entries used for:

- detailed investigation
- drill-down
- debugging
- displaying individual logs
- future filtering

Example:

```text
13:00:01 app1 object not found
13:00:04 app1 object not found
13:00:09 app1 object not found
```

## Aggregations

Precomputed information used for:

- dashboards
- charts
- counts
- grouping
- fast statistics queries

Example:

```text
11:00 → 0
12:00 → 5
13:00 → 51
```

The raw events should be treated as the source of truth.

Aggregations should be treated as derived data.

Architecture:

```text
Incoming log
     │
     ▼
  Parse
     │
     ▼
Normalize
     │
     ├───────────────┐
     ▼               ▼
Raw storage       Aggregation
     │               │
     ▼               ▼
Drill-down         Charts
```

---

# 6. Database Strategy

Do not start with Redis.

Use progressively more realistic implementations:

```text
Phase 1
Memory

Phase 2
Redis

Phase 3
Evaluate whether another database is more appropriate
```

Redis can work, but the requirements are increasingly analytical:

- time-range queries
- application filtering
- error filtering
- hourly aggregation
- arbitrary future fields
- individual event retrieval
- retention
- pagination

The purpose of the early implementation is to learn the domain and query patterns before optimizing the persistence layer.

---

# 7. Suggested Domain Model

Start with:

```go
type LogEntry struct {
    ID        string
    Timestamp time.Time
    App       string
    Container string          // e.g. "router", "web-1"
    Level     LogLevel        // DEBUG, INFO, WARN, ERROR
    Message   string
    ErrorCode string          // e.g. "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT"
    Fields    map[string]any
}
```

The `Fields` map allows custom structured logging data to be preserved.

Example:

```json
{
  "timestamp": "2026-09-24T12:34:56Z",
  "app": "payments",
  "level": "error",
  "message": "object not found",
  "fields": {
    "userId": "123",
    "orderId": "456",
    "requestId": "abc",
    "service": "checkout"
  }
}
```

Do not implement arbitrary-field indexing immediately.

Initially, just preserve the fields.

### Real-World Example: Scalingo Router Log

```text
2026-06-24 15:02:19.446989072 +0200 CEST[router] method=GET path="/password?callbackUrl=https%3A%2F%2Fwww.hedia.com%2Faccount%2Fcallback" host=id.hedia.com request_id=cc44edd4-6926-46eb-9f7e-da9a91239549 container=web-2 from="87.54.121.58" protocol=https status=200 duration=0.030s bytes=5332 referer="https://id.hedia.com/account?callbackUrl=https%3A%2F%2Fwww.hedia.com%2Faccount%2Fcallback" user_agent="Mozilla/5.0 (Linux; Android 16; SM-S911B Build/BP4A.251205.006; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/149.0.7827.91 Mobile Safari/537.36"
```

**Key parsing details:**

- **Timestamp**: `2026-06-24 15:02:19.446989072 +0200 CEST`
- **Source**: `[router]`
- **Format**: `logfmt` / key-value format (`method=GET`, `path=...`, `status=200`, `duration=0.030s`, etc.)
- **Level derivation**: Derived from `status` (e.g. `5xx` -> `LevelError`, `4xx` -> `LevelWarn`, `2xx/3xx` -> `LevelInfo`)
- **Fields**: Key-value pairs map cleanly into `Fields map[string]any`

### Real-World Example: Custom Application Error Log

Raw log line:

```text
2025-09-08 13:12:58.867176204 +0200 CEST[web-1] {"app":{"name":"demo-app","version":"3.4.2"},"data":{"error":{"code":"FETCH_CLIENT_ERROR_REQUEST_TIMEOUT","message":"Request aborted after 5000 milliseconds","requestMethod":"GET","requestUrl":"https://data.hedia.dev/api/v1/users/40b7f973-1038-4c38-81e4-e8f3de0113da/objects?order=desc","stack":"Error: Request aborted after 5000 milliseconds\n at ClientRequest.parseResponseError (/app/node_modules/@hedia/fetch/src/index.ts:405:10)\n at process.processTicksAndRejections (node:internal/process/task_queues:105:5)\n at async DataServiceClient.listObjects (/app/src/integrations/dataService.ts:76:20)\n at async dashboardRequestListener (/app/src/main.ts:310:20)\n at async Object.listener (/app/node_modules/@hedia/http/src/index.ts:33:4)\n at async <anonymous> (/app/node_modules/@hedia/http/src/routing.ts:54:3)\n at async Server.errorRequestListener (/app/src/main.ts:150:5)"}},"level":"error","message":"Error processing request","origin":{"column":30,"file":"src/main.ts","line":158},"params":[],"timestamp":1757329978852}
```

Parsed JSON payload:

```json
{
  "app": {
    "name": "demo-app",
    "version": "3.4.2"
  },
  "data": {
    "error": {
      "code": "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT",
      "message": "Request aborted after 5000 milliseconds",
      "requestMethod": "GET",
      "requestUrl": "https://data.hedia.dev/api/v1/users/40b7f973-1038-4c38-81e4-e8f3de0113da/objects?order=desc",
      "stack": "Error: Request aborted after 5000 milliseconds\n..."
    }
  },
  "level": "error",
  "message": "Error processing request",
  "origin": {
    "column": 30,
    "file": "src/main.ts",
    "line": 158
  },
  "params": [],
  "timestamp": 1757329978852
}
```

**Key parsing details:**

- **Envelope / Prefix**: Scalingo drain timestamp (`2025-09-08 13:12:58.867176204 +0200 CEST`) + container tag (`[web-1]`).
- **Application**: Extracted from `app.name` (`demo-app`) with version `3.4.2`.
- **Level**: Explicit `"level": "error"` (maps directly to `LevelError`).
- **Message**: `"Error processing request"`.
- **Error Aggregation Key**: `data.error.code` (`FETCH_CLIENT_ERROR_REQUEST_TIMEOUT`) is the exact stable category code needed for error grouping and aggregation (see Section 8)!
- **Fields**: `data`, `origin`, and `params` map into `Fields map[string]any`.

---

# 8. Error Modeling

Do not rely entirely on the human-readable error message for aggregation.

Prefer eventually having something like:

```json
{
  "level": "error",
  "message": "order 123 was not found",
  "error": {
    "type": "ObjectNotFound",
    "message": "order 123 was not found"
  }
}
```

Then:

```text
error.type
```

can be used for aggregation:

```text
ObjectNotFound → 56 occurrences
```

while the original message remains available in the raw event.

This avoids treating:

```text
user 123 not found
user 456 not found
user 789 not found
```

as three completely different error categories.

---

# 9. Learning Plan

The project should be built through a series of exercises.

---

# Exercise 1 — Create a Tiny Go Service

## Goal

Learn:

- Go modules
- packages
- `main`
- `net/http`
- HTTP handlers
- JSON
- basic error handling

Create:

```text
logwatch/
├── go.mod
└── main.go
```

Implement:

```http
GET /health
```

Response:

```json
{
  "status": "ok"
}
```

And:

```http
GET /version
```

Response:

```json
{
  "version": "0.1.0"
}
```

## Rules

Do not use a web framework.

Use:

```go
net/http
```

## Exercises

- [x] Start an HTTP server (configured with timeouts).
- [x] Create two handlers:
  - [x] `GET /health` (returns `{"status": "ok"}`)
  - [x] `GET /version` (returns `{"version": "0.1.0"}`)
- [x] Return JSON.
- [x] Set `Content-Type: application/json`.
- [x] Handle server startup errors.
- [x] Experiment with `http.Server`.

---

# Exercise 2 — Model a Log Entry & Typed Log Levels

## Goal

Learn:

- Custom Go types & enum pattern (`type LogLevel string`)
- Package-level constants (`const`)
- Structs and exported fields
- Methods with value receivers
- Unit testing with table-driven tests (`testing.T`)

## Design

Create:

```text
internal/logs/log.go
```

### 1. Typed Log Level & Constants

```go
type LogLevel string

const (
    LevelDebug LogLevel = "DEBUG"
    LevelInfo  LogLevel = "INFO"
    LevelWarn  LogLevel = "WARN"
    LevelError LogLevel = "ERROR"
)
```

### 2. Domain Model

```go
type LogEntry struct {
    ID        string
    Timestamp time.Time
    App       string
    Container string          // e.g. "router", "web-1"
    Level     LogLevel
    Message   string
    ErrorCode string          // e.g. "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT"
    Fields    map[string]any
}
```

### 3. Methods & Helpers

- `func ParseLogLevel(s string) (LogLevel, error)`: Normalizes case-insensitively (`"error"` -> `LevelError`) and returns an error for unknown levels.
- `func (l LogEntry) IsError() bool`: Checks if `l.Level == LevelError`.

## Exercises

- [x] Create `internal/logs/log.go` with package `logs`.
- [x] Define `LogLevel` and its `const` values.
- [x] Define `LogEntry` struct using `LogLevel`, `Container`, and `ErrorCode`.
- [x] Implement `ParseLogLevel(s string) (LogLevel, error)`.
- [x] Implement `(l LogEntry) IsError() bool`.
- [ ] Write unit tests in `internal/logs/log_test.go` covering all levels, case insensitivity, and invalid strings.// will be done much later

---

# Exercise 3 — Real-World Log Parsing

## Goal

Learn:

- String manipulation & envelope parsing (extracting Scalingo prefixes)
- Nested JSON unmarshaling (`json.Unmarshal`) into typed Go structs
- Handling Unix epoch millisecond timestamps (`time.UnixMilli`)
- Key-value parsing for `logfmt` router logs
- Go error propagation & input validation

## Architecture: Two-Stage Parser

Real Scalingo lines look like:

- App log: `2025-09-08 13:12:58... [web-1] {"app":{"name":"demo-app"}, ...}`
- Router log: `2026-06-24 15:02:19... [router] method=GET ... status=200`

## Parsing Policy: Never Lose a Log

Stage-2 parsers never return an error. They parse as much as they can and
fall back for whatever they can't:

- Unknown or missing level -> `LevelUnknown` (new constant in `log.go`)
- The original line is always kept in `LogEntry.Raw`
- Low-level helpers that can genuinely fail (e.g. `ParseLogLevel`) stay
  strict and return an error; the stage-2 parser catches it and applies the
  fallback. Low-level code reports problems, high-level code chooses policy.

`LevelUnknown` keeps malformed lines queryable ("how many lines had no valid
level?") without distorting error and warning statistics.

### 1. Stage 1 — Envelope Parser

Extracts the common Scalingo metadata prefix:

```go
type Envelope struct {
    Timestamp time.Time
    Container string // e.g. "web-1", "router"
    Payload   string // the raw inner content
}

func ParseEnvelope(line string) (Envelope, error)
```

### 2. Stage 2A — App JSON Log Parser

Handles the nested JSON payload from application containers:

```go
type AppLogPayload struct {
    App struct {
        Name    string `json:"name"`
        Version string `json:"version"`
    } `json:"app"`
    Level     string         `json:"level"`
    Message   string         `json:"message"`
    Timestamp int64          `json:"timestamp"` // Unix epoch milliseconds
    Data      struct {
        Error struct {
            Code    string `json:"code"`
            Message string `json:"message"`
            Stack   string `json:"stack"`
        } `json:"error"`
    } `json:"data"`
    Origin map[string]any `json:"origin"`
    Params []any          `json:"params"`
}

func ParseAppLog(env Envelope) LogEntry
```

Fallbacks:

| Problem                               | Fallback                                         |
| ------------------------------------- | ------------------------------------------------ |
| Invalid JSON (plain text, stack trace) | `Message = env.Payload`, `Level = LevelUnknown` |
| Unknown or missing `level`            | `Level = LevelUnknown`                           |
| Missing `timestamp` (would be 1970)   | `Timestamp = env.Timestamp`                      |

### 3. Stage 2B — Router Logfmt Parser

Handles the key-value logs from `[router]`.

Helper (unexported, never fails):

```go
func parseLogfmt(s string) map[string]string
```

Lenient parsing rules — best effort, never drop data:

| Input          | Result                  | Rule                                   |
| -------------- | ----------------------- | -------------------------------------- |
| `a=`           | `{"a": ""}`             | empty value                            |
| `=1`           | `{}`                    | pair without key is skipped            |
| `a="unclosed`  | `{"a": "unclosed"}`     | unclosed quote runs to end of string   |
| `a="2=3"`      | `{"a": "2=3"}`          | `=` inside quotes is part of the value |
| `a=1 a=2`      | `{"a": "2"}`            | duplicate key: last wins               |

Accepted trade-off: lenient parsing can silently produce wrong fields
(e.g. `a="unclosed b=2` → `{"a": "unclosed b=2"}`). The safeguard is that
the original line is always kept in `LogEntry.Raw`, so a bad parse can be
inspected and re-parsed later.

```go
func ParseRouterLog(env Envelope) LogEntry
```

- Parses `status=...`, `method=...`, `path=...`, `duration=...`
- Derives `Level`:
  - `status >= 500` -> `LevelError`
  - `status >= 400` -> `LevelWarn`
  - Otherwise -> `LevelInfo`
  - Missing or non-numeric `status` -> `LevelUnknown`
- Keeps the raw `status` string in `Fields`, even when the level falls back

### 4. Dispatcher

```go
func ParseLog(rawLine string) (LogEntry, error)
```

Coordinates the pipeline:

1. `ParseEnvelope(rawLine)`
2. If `env.Container == "router"`, calls `ParseRouterLog(env)`
3. Otherwise calls `ParseAppLog(env)` (it handles non-JSON payloads itself)
4. Sets `entry.Raw = rawLine`
5. Returns the unified `LogEntry`

Open decision: `ParseEnvelope` is now the only parser that can fail (missing
brackets, bad timestamp). Decide whether such lines are dropped or stored with
a fallback (e.g. receive time as timestamp, `LevelUnknown`, plus `Raw`) — and
therefore whether `ParseLog` returns an `error` at all.

## Exercises

- [x] Implement `ParseEnvelope(line string) (Envelope, error)`
- [x] Implement `ParseAppLog(env Envelope) (LogEntry, error)` with `time.UnixMilli`
- [x] Implement `parseLogfmt` with table-driven tests
- [x] Make `parseLogfmt` lenient: signature `map[string]string` (no `error`), rules above applied, single store guarded by `key != ""` (16 cases passing)
- [ ] Add `LevelUnknown` constant and `Raw string` field to `LogEntry` in `log.go`
- [ ] Make `ParseAppLog` lenient: signature `LogEntry` (no `error`), apply the fallbacks above, add tests (invalid JSON, unknown level, missing timestamp)
- [ ] Implement `ParseRouterLog(env Envelope) LogEntry` with `LevelUnknown` fallback
- [ ] Implement top-level `ParseLog(rawLine string) (LogEntry, error)`
- [ ] Write unit tests verifying parsing of real Scalingo router & app error samples

---

# Exercise 4 — Create a Repository

Define:

```go
type LogRepository interface {
    Save(ctx context.Context, log LogEntry) error
    Find(ctx context.Context, query LogQuery) ([]LogEntry, error)
}
```

Create an in-memory implementation:

```go
type MemoryLogRepository struct {
    logs []LogEntry
}
```

The application should now look like:

```text
HTTP
  ↓
Service
  ↓
LogRepository
  ↓
MemoryLogRepository
```

## Exercises

Implement:

```go
Save()
Find()
```

Support at least:

```text
From
To
App
```

Learn when and why interfaces are useful.

Do not create interfaces for every object.

Use interfaces at meaningful boundaries.

---

# Exercise 5 — Learn Goroutines and Channels

Create a fake producer.

```text
Producer
    ↓
channel
    ↓
consumer
    ↓
repository
```

Example:

```go
logs := make(chan LogEntry)

go producer(logs)

for log := range logs {
    repository.Save(ctx, log)
}
```

Learn:

```go
go func() {}
```

```go
chan LogEntry
```

```go
close(logs)
```

## Exercise

Generate 1,000 fake logs.

Send them through a channel.

Store them in the memory repository.

---

# Exercise 6 — Build a Worker Pool

Create multiple workers:

```text
              ┌── worker 1
              │
producer ─── channel ── worker 2
              │
              ├── worker 3
              │
              └── worker 4
```

Create:

```go
type Processor struct {
    workers int
}
```

Implement:

```go
func (p *Processor) Start(ctx context.Context)
```

Learn:

- goroutines
- channels
- `sync.WaitGroup`
- worker pools
- concurrency
- backpressure

## Experiment

Process:

```text
100,000 logs
```

Compare:

```text
1 worker
2 workers
4 workers
8 workers
```

Measure throughput.

---

# Exercise 7 — Learn Context

Introduce:

```go
context.Context
```

Every important operation should eventually accept a context:

```go
Save(ctx context.Context, ...)
Find(ctx context.Context, ...)
```

Practice:

```go
ctx, cancel := context.WithTimeout(...)
defer cancel()
```

Learn how cancellation flows through:

```text
HTTP
 ↓
Service
 ↓
Repository
 ↓
Redis
```

---

# Exercise 8 — Build the Ingestion API

Create:

```http
POST /ingest
```

Request:

```json
{
  "app": "app1",
  "timestamp": "2026-09-24T12:00:23Z",
  "message": "object not found"
}
```

Flow:

```text
HTTP request
     ↓
Decode JSON
     ↓
Validate
     ↓
Create LogEntry
     ↓
Send to channel
     ↓
Return response
```

The HTTP handler should not necessarily write directly to the database.

Instead:

```text
HTTP
 ↓
channel
 ↓
processor
 ↓
storage
```

---

# Exercise 9 — Asynchronous Ingestion

Create:

```go
type Ingestor struct {
    queue chan LogEntry
}
```

The HTTP handler calls:

```go
ingestor.Enqueue(log)
```

Workers consume:

```go
for log := range queue {
    repository.Save(...)
}
```

## Important design exercise

What happens when the queue is full?

Possible strategies:

```text
Queue full
    ↓
Block
```

or:

```text
Queue full
    ↓
Reject request
```

or:

```text
Queue full
    ↓
Drop log
```

For this application, dropping logs may be unacceptable.

Decide explicitly which behavior you want.

---

# Exercise 10 — Build a Fake Scalingo Source

Do not connect to Scalingo yet.

Define:

```go
type LogSource interface {
    Stream(ctx context.Context, app Application, out chan<- RawLog) error
}
```

Create:

```go
type FakeLogSource struct {}
```

Generate realistic logs for:

```text
app1
app2
app3
...
app70
```

Generate:

```text
INFO
WARN
ERROR
```

and different messages.

Architecture:

```text
FakeLogSource
      │
      ▼
   channel
      │
      ▼
 processors
      │
      ▼
 repository
```

---

# Exercise 11 — Multiple Applications

Create:

```go
type Application struct {
    ID   string
    Name string
}
```

Create:

```go
type ApplicationRegistry struct {
    applications []Application
}
```

Simulate 70 applications:

```text
app1 ─┐
app2 ─┤
app3 ─┤
...   ├──> log stream
app70 ┘
```

## Exercise

Run concurrent streams:

```go
for _, app := range apps {
    go source.Stream(ctx, app, output)
}
```

Learn:

- concurrent producers
- shared channels
- cancellation
- lifecycle management

---

# Exercise 12 — Design the Query Model

Create:

```go
type LogQuery struct {
    From      time.Time
    To        time.Time
    App       string
    Level     string
    Error     string
    Limit     int
    Cursor    string
}
```

Eventually support:

```http
GET /logs?from=...&to=...
```

and:

```http
GET /logs?from=...&to=...&app=app1
```

and:

```http
GET /logs?from=...&to=...&app=app1&error=object-not-found
```

Keep query parsing separate from repository implementation.

---

# Exercise 13 — Aggregation

Create:

```go
type HourlyBucket struct {
    Hour  time.Time
    Count int
}
```

Given:

```text
12:03 error
12:05 error
12:40 error
13:01 error
13:20 error
13:50 error
```

return:

```json
[
  {
    "hour": "2026-09-24T12:00:00Z",
    "count": 3
  },
  {
    "hour": "2026-09-24T13:00:00Z",
    "count": 3
  }
]
```

## Important requirement

Include hours with zero events.

For example:

```json
[
  {
    "hour": "2026-09-24T11:00:00Z",
    "count": 0
  },
  {
    "hour": "2026-09-24T12:00:00Z",
    "count": 3
  },
  {
    "hour": "2026-09-24T13:00:00Z",
    "count": 3
  }
]
```

This makes chart rendering easier.

---

# Exercise 14 — Add Statistics APIs

Implement:

```http
GET /stats/apps
```

Example:

```json
[
  {
    "app": "app1",
    "count": 1432
  },
  {
    "app": "app2",
    "count": 912
  }
]
```

Implement:

```http
GET /stats/errors
```

Example:

```json
[
  {
    "app": "app1",
    "error": "object not found",
    "count": 56
  }
]
```

Implement:

```http
GET /stats/hourly
```

Example:

```json
[
  {
    "hour": "2026-09-24T11:00:00Z",
    "count": 0
  },
  {
    "hour": "2026-09-24T12:00:00Z",
    "count": 5
  },
  {
    "hour": "2026-09-24T13:00:00Z",
    "count": 51
  }
]
```

---

# Exercise 15 — Introduce Redis

Only after the in-memory implementation works should Redis be introduced.

Keep:

```go
type LogRepository interface {
    Save(ctx context.Context, log LogEntry) error
    Find(ctx context.Context, query LogQuery) ([]LogEntry, error)
}
```

Implement:

```text
MemoryLogRepository
RedisLogRepository
```

The rest of the application should not care which one is being used.

---

# 10. Possible Redis Data Model

The exact Redis structure should be validated against real query requirements and expected volume.

One possible approach is to maintain a time index for raw events.

Conceptually:

```text
logs:2026-09-24
```

could contain:

```text
timestamp → log ID
```

while:

```text
log:{id}
```

contains the complete event.

For example:

```text
logs:2026-09-24
    │
    ├── timestamp → log-123
    ├── timestamp → log-124
    └── timestamp → log-125

log:123 → complete event
log:124 → complete event
log:125 → complete event
```

This allows time-range lookup followed by retrieval of the individual events.

---

# 11. Aggregated Redis Data

Maintain derived statistics separately from raw events.

Conceptually:

```text
stats:{date}:{app}:{error}
```

could contain hourly counters:

```text
00 → 0
01 → 0
02 → 0
...
11 → 0
12 → 5
13 → 51
...
23 → 0
```

The exact schema should be designed after the query API is finalized.

The important principle is:

```text
Raw events = source of truth

Aggregations = derived data
```

---

# Exercise 16 — Scalingo Integration

Once the application works with a fake source, replace it with a real Scalingo implementation.

Create:

```go
type ScalingoLogSource struct {
    client *scalingo.Client
}
```

Implement:

```go
type LogSource interface {
    Stream(ctx context.Context, app Application, out chan<- RawLog) error
}
```

The rest of the application should remain unchanged.

Before implementing this, investigate the current Scalingo options:

- live log streaming
- log drains
- log APIs
- Go SDK capabilities

Scalingo supports external log drains, including syslog over UDP, TCP, and TCP+TLS.

The service should not assume that polling is necessarily the best ingestion mechanism.

---

# Exercise 17 — Connection Retry and Backoff

Real log streams can fail.

Implement:

```go
func streamWithRetry(ctx context.Context) error
```

Possible retry schedule:

```text
attempt 1
    ↓
1 second

attempt 2
    ↓
2 seconds

attempt 3
    ↓
4 seconds

attempt 4
    ↓
8 seconds
```

Add a maximum retry delay.

Learn:

```go
time.NewTimer
```

and:

```go
select {
case <-ctx.Done():
    return ctx.Err()

case <-timer.C:
    // retry
}
```

---

# Exercise 18 — Graceful Shutdown

The service should handle:

```text
SIGTERM
```

Shutdown sequence:

```text
SIGTERM
   │
   ▼
Stop accepting new work
   │
   ▼
Stop ingestion
   │
   ▼
Finish queued logs
   │
   ▼
Close storage connections
   │
   ▼
Exit
```

Learn:

```go
signal.NotifyContext(...)
```

and:

```go
context.WithCancel(...)
```

---

# Exercise 19 — Time Handling

Define exactly what "today" means.

Possible interpretation:

```text
Europe/Copenhagen
```

while stored timestamps are normalized to UTC.

Use:

```go
time.Time
```

throughout the application.

Test:

- midnight
- previous day
- next day
- month boundaries
- year boundaries
- daylight-saving transitions
- UTC conversion

The API should have explicit timezone semantics.

---

# Exercise 20 — Testing

Use Go's native testing system.

Tests should exist at multiple levels.

## Unit tests

Test:

```text
Parser
Validation
Aggregation
Time bucketing
Error classification
```

## Repository tests

Test:

```text
MemoryRepository
RedisRepository
```

## Service tests

Test:

```text
LogService
AggregationService
```

## HTTP tests

Test:

```text
GET /logs
GET /stats/apps
GET /stats/errors
GET /stats/hourly
POST /ingest
```

## Integration tests

Eventually:

```text
HTTP
 ↓
Service
 ↓
Redis
 ↓
Query
```

---

# 12. Eventual API

## Health

```http
GET /health
```

## Ingestion

```http
POST /ingest
```

This may eventually be internal/private.

## Raw logs

```http
GET /logs
```

Parameters:

```text
from
to
app
level
error
limit
cursor
```

Example:

```http
GET /logs?from=2026-09-24T00:00:00Z&to=2026-09-25T00:00:00Z&app=app1&error=object-not-found
```

## Application statistics

```http
GET /stats/apps
```

## Error statistics

```http
GET /stats/errors
```

## Hourly statistics

```http
GET /stats/hourly
```

---

# 13. Eventual Frontend

The frontend can consume the API without knowing anything about Scalingo or Redis.

Example dashboard:

```text
Today
─────────────────────────────────────

Total errors                    1432

Applications

app1                             932
app2                             311
app3                             189


Errors

object not found                 56
connection timeout               42
invalid user                     17


object not found
─────────────────────────────────────

11:00  0
12:00  █████
13:00  █████████████████████████████████████████████████
```

Clicking the `13:00` bucket would query:

```http
GET /logs
    ?from=13:00
    &to=14:00
    &app=app1
    &error=object-not-found
```

and display the individual events.

---

# 14. Eventual Package Structure

Do not create this entire structure on day one.

Let it evolve.

Eventually it could look approximately like:

```text
logwatch/
│
├── cmd/
│   └── server/
│       └── main.go
│
├── internal/
│   │
│   ├── logs/
│   │   ├── model.go
│   │   ├── parser.go
│   │   ├── service.go
│   │   └── repository.go
│   │
│   ├── ingestion/
│   │   ├── service.go
│   │   ├── worker.go
│   │   └── source.go
│   │
│   ├── scalingo/
│   │   └── source.go
│   │
│   ├── aggregation/
│   │   └── service.go
│   │
│   ├── storage/
│   │   ├── memory/
│   │   │   └── repository.go
│   │   └── redis/
│   │       └── repository.go
│   │
│   └── http/
│       ├── handler.go
│       ├── logs.go
│       └── stats.go
│
├── migrations/
│
├── go.mod
└── go.sum
```

Do not over-engineer the project at the beginning.

Start small and introduce packages when they become useful.

---

# 15. Learning Progression

The complete learning path:

```text
01. Go project + HTTP
02. Structs + methods
03. JSON + validation
04. Errors
05. Interfaces
06. Unit tests
07. Goroutines
08. Channels
09. Worker pools
10. Context
11. HTTP ingestion
12. Graceful shutdown
13. Fake log source
14. Multiple concurrent sources
15. Aggregation
16. Query API
17. Redis
18. Scalingo integration
19. Retry/backoff
20. Observability
21. Integration tests
22. Performance testing
23. Docker
24. Production deployment
```

---

# 16. Milestones

## Milestone 1 — Minimal Go Service

Build:

```text
FakeLogSource
      ↓
     chan
      ↓
   Processor
      ↓
MemoryRepository
      ↓
GET /logs
```

Requirements:

- 5 fake applications
- 1,000 fake logs
- 3 worker goroutines
- in-memory storage
- HTTP endpoint

---

## Milestone 2 — Aggregation

Add:

```text
                    ┌── raw logs
                    │
Fake source → queue ┤
                    │
                    └── hourly aggregation
```

Implement:

```http
GET /stats/hourly
```

---

## Milestone 3 — Scalingo

Replace:

```text
FakeLogSource
```

with:

```text
ScalingoLogSource
```

without changing the rest of the application.

---

## Milestone 4 — Redis

Replace:

```text
MemoryRepository
```

with:

```text
RedisRepository
```

without changing the service layer.

---

## Milestone 5 — Reliability

The system should survive:

- Scalingo connection failure
- Redis failure
- malformed log
- worker failure
- HTTP client disconnect
- SIGTERM
- full ingestion queue
- temporary network failure

---

# 17. First Exercise

Do **not** start with Scalingo.

Build this first:

```text
FakeLogSource
      │
      ▼
   channel
      │
      ▼
  3 workers
      │
      ▼
MemoryRepository
      │
      ▼
GET /logs
```

Requirements:

```text
5 fake applications

1,000 generated logs

Each log contains:

    ID
    timestamp
    app
    level
    message
```

The application should:

1. Start an HTTP server.
2. Start a fake log producer.
3. Generate logs continuously.
4. Send logs through a channel.
5. Process logs using 3 worker goroutines.
6. Store them in memory.
7. Expose `GET /logs`.
8. Return the stored logs as JSON.
9. Have unit tests for the repository.
10. Have unit tests for the processor.

Do not use:

- Redis
- Scalingo
- a web framework
- a message broker
- a frontend

The goal is to learn the Go fundamentals while building the first vertical slice of the real system.

---

# 18. Go Concepts This Project Will Teach

By the end of the project you should be comfortable with:

- Go modules
- packages
- exported/unexported identifiers
- structs
- methods
- interfaces
- pointers
- slices
- maps
- `any`
- errors
- error wrapping
- JSON
- HTTP servers
- HTTP clients
- `context.Context`
- goroutines
- channels
- buffered channels
- `select`
- `sync.WaitGroup`
- mutexes
- worker pools
- cancellation
- graceful shutdown
- timers
- retries
- backoff
- `io.Reader`
- `io.ReadCloser`
- testing
- table-driven tests
- benchmarks
- race detection
- dependency management
- Redis integration
- application lifecycle
- concurrency design

Most importantly, these concepts will be learned in the context of a real backend system rather than as isolated Go exercises.

---

# 19. Recommended Development Philosophy

For each exercise:

1. Read the requirements.
2. Implement it yourself.
3. Run it.
4. Write tests.
5. Intentionally break it.
6. Observe what happens.
7. Fix it.
8. Only then move to the next exercise.

Avoid copying a complete implementation.

The goal is not merely to finish the log service.

The goal is to finish the log service **while developing an intuition for how Go works**.

The project should progressively evolve from:

```text
small Go program
```

into:

```text
concurrent Go service
```

and eventually into:

```text
production-style log ingestion and analytics backend
```
