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
  `ParseLog(rawLine string, receivedAt time.Time) LogEntry`, no `error`. —
  Deterministic tests; re-parsing old lines from `Raw` keeps their original
  time. *Not chosen:* `time.Now()` inside the parser; leaving it at zero.
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
