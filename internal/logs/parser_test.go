package logs

import (
	"maps"
	"reflect"
	"testing"
	"time"
)

// Timestamp at the start of every Scalingo line.
const timePrefix = "2025-09-08 13:12:58.867176204 +0200 CEST"

// Prefix of a real Scalingo line for container web-1.
const appLinePrefix = timePrefix + " [web-1] "

// 2025-09-08 13:12:58.867176204 +0200 CEST as an instant.
var envTime = time.Date(2025, 9, 8, 11, 12, 58, 867176204, time.UTC)

// Real sample from a Scalingo app container (stack trace shortened).
const appLogErrorPayload = `{"app":{"name":"demo-app","version":"3.4.2"},"data":{"error":{"code":"FETCH_CLIENT_ERROR_REQUEST_TIMEOUT","message":"Request aborted after 5000 milliseconds","stack":"Error: Request aborted after 5000 milliseconds\n at ClientRequest.parseResponseError (/app/node_modules/@hedia/fetch/src/index.ts:405:10)"}},"level":"error","message":"Error processing request","origin":{"column":30,"file":"src/main.ts","line":158},"params":[],"timestamp":1757329978852}`

func TestParseEnvelope(t *testing.T) {
	routerLine := `2026-06-24 15:02:19.446989072 +0200 CEST [router] method=GET path="/password" status=200 duration=0.030s`
	appLine := appLinePrefix + appLogErrorPayload
	payloadWithBrackets := appLinePrefix + `{"params":["a","b"],"message":"see [docs]"}`

	data := []struct {
		name    string
		input   string
		want    Envelope
		wantErr bool
	}{
		{"app line", appLine, Envelope{
			Timestamp: envTime,
			Container: "web-1",
			Payload:   appLogErrorPayload,
			Raw:       appLine,
		}, false},
		{"router line", routerLine, Envelope{
			Timestamp: time.Date(2026, 6, 24, 13, 2, 19, 446989072, time.UTC),
			Container: "router",
			Payload:   `method=GET path="/password" status=200 duration=0.030s`,
			Raw:       routerLine,
		}, false},
		{"brackets inside payload belong to payload", payloadWithBrackets, Envelope{
			Timestamp: envTime,
			Container: "web-1",
			Payload:   `{"params":["a","b"],"message":"see [docs]"}`,
			Raw:       payloadWithBrackets,
		}, false},
		{"payload whitespace is trimmed", appLinePrefix + "  hello  ", Envelope{
			Timestamp: envTime,
			Container: "web-1",
			Payload:   "hello",
			Raw:       appLinePrefix + "  hello  ",
		}, false},
		{"empty payload", appLinePrefix, Envelope{
			Timestamp: envTime,
			Container: "web-1",
			Raw:       appLinePrefix,
		}, false},
		{"no zone abbreviation", "2025-09-08 13:12:58.867176204 +0200 [web-1] hello", Envelope{
			Timestamp: envTime,
			Container: "web-1",
			Payload:   "hello",
			Raw:       "2025-09-08 13:12:58.867176204 +0200 [web-1] hello",
		}, false},
		{"no container", timePrefix + " some text", Envelope{
			Timestamp: envTime,
			Payload:   "some text",
			Raw:       timePrefix + " some text",
		}, false},
		{"no container, payload has brackets", timePrefix + ` {"params":["a"]}`, Envelope{
			Timestamp: envTime,
			Payload:   `{"params":["a"]}`,
			Raw:       timePrefix + ` {"params":["a"]}`,
		}, false},
		{"timestamp only", timePrefix, Envelope{
			Timestamp: envTime,
			Raw:       timePrefix,
		}, false},
		{"no space after container", timePrefix + " [web-1]hello", Envelope{
			Timestamp: envTime,
			Container: "web-1",
			Payload:   "hello",
			Raw:       timePrefix + " [web-1]hello",
		}, false},
		// Without a parsable timestamp nothing in the line is trusted:
		// ParseEnvelope returns an envelope with only Raw set, together with
		// an error. Real Scalingo lines always carry a timestamp, so this is
		// a safety net, not a path that needs structured parsing.
		{"empty line", ``, Envelope{}, true},
		{"bad timestamp", `yesterday [web-1] hello`, Envelope{
			Raw: `yesterday [web-1] hello`,
		}, true},
		{"date without time", `2025-09-08 [web-1] hello`, Envelope{
			Raw: `2025-09-08 [web-1] hello`,
		}, true},
		{"no timestamp at all", `[web-1] hello`, Envelope{
			Raw: `[web-1] hello`,
		}, true},
		{"JSON only", `{"level":"error","message":"boom"}`, Envelope{
			Raw: `{"level":"error","message":"boom"}`,
		}, true},
		{"Raw keeps surrounding whitespace", "  plain text  ", Envelope{
			Raw: "  plain text  ",
		}, true},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			result, err := ParseEnvelope(d.input)
			if (err != nil) != d.wantErr {
				t.Errorf("ParseEnvelope(%q) error = %v, wantErr %v", d.input, err, d.wantErr)
			}
			// The envelope is checked even on error: Raw must never be lost.
			// time.Parse gives the CEST timestamp a synthetic location, so compare
			// instants with Equal instead of comparing whole structs.
			if !result.Timestamp.Equal(d.want.Timestamp) {
				t.Errorf("Timestamp = %v, want %v", result.Timestamp, d.want.Timestamp)
			}
			if result.Container != d.want.Container {
				t.Errorf("Container = %q, want %q", result.Container, d.want.Container)
			}
			if result.Payload != d.want.Payload {
				t.Errorf("Payload = %q, want %q", result.Payload, d.want.Payload)
			}
			if result.Raw != d.want.Raw {
				t.Errorf("Raw = %q, want %q", result.Raw, d.want.Raw)
			}
		})
	}
}

// parseTimestamp reads "date clock offset [ZONE]" from the start of a line.
// rest is what follows, with the single separating space removed.
// On error, t is zero and rest is "".
func TestParseTimestamp(t *testing.T) {
	data := []struct {
		name     string
		input    string
		wantTime time.Time
		wantRest string
		wantErr  bool
	}{
		{"full prefix with zone and container", timePrefix + " [web-1] hello", envTime, "[web-1] hello", false},
		{"no zone abbreviation", "2025-09-08 13:12:58.867176204 +0200 [web-1] hello", envTime, "[web-1] hello", false},
		{"timestamp with zone only", timePrefix, envTime, "", false},
		{"timestamp without zone only", "2025-09-08 13:12:58.867176204 +0200", envTime, "", false},
		{"UTC zone", "2025-09-08 11:12:58.867176204 +0000 UTC hello", envTime, "hello", false},
		{"fewer fractional digits", "2025-09-08 13:12:58.8 +0200 CEST x",
			time.Date(2025, 9, 8, 11, 12, 58, 800000000, time.UTC), "x", false},
		{"no fractional seconds", "2025-09-08 13:12:58 +0200 CEST x",
			time.Date(2025, 9, 8, 11, 12, 58, 0, time.UTC), "x", false},
		{"payload brackets are not touched", timePrefix + ` {"params":["a"]}`, envTime, `{"params":["a"]}`, false},

		// The zone word is skipped only when it looks like one: 3-5 uppercase
		// letters, standing alone.
		{"text glued to zone is not a zone", timePrefix + "abc", envTime, "CESTabc", false},
		{"lowercase word is not a zone", "2025-09-08 13:12:58.867176204 +0200 hello world", envTime, "hello world", false},
		{"long uppercase word is not a zone", "2025-09-08 13:12:58.867176204 +0200 WARNING disk full", envTime, "WARNING disk full", false},
		// Accepted trade-off: a short uppercase word right after the offset is
		// indistinguishable from a zone and is skipped. Raw still keeps it.
		{"short uppercase word is taken as zone", "2025-09-08 13:12:58.867176204 +0200 WARN disk full", envTime, "disk full", false},

		{"empty line", ``, time.Time{}, "", true},
		{"no timestamp", `[web-1] hello`, time.Time{}, "", true},
		{"word instead of date", `yesterday [web-1] hello`, time.Time{}, "", true},
		{"date without clock", `2025-09-08 [web-1] hello`, time.Time{}, "", true},
		{"missing offset", "2025-09-08 13:12:58.867176204", time.Time{}, "", true},
		{"zone instead of offset", "2025-09-08 13:12:58.867176204 CEST [web-1] hello", time.Time{}, "", true},
		{"impossible date", "2025-13-45 13:12:58 +0200 CEST x", time.Time{}, "", true},
		{"leading whitespace", "  " + timePrefix + " x", time.Time{}, "", true},
		{"lowercase short word is not a zone", "2025-09-08 13:12:58.867176204 +0200 info disk full", envTime, "info disk full", false},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			gotTime, gotRest, err := parseTimestamp(d.input)
			if (err != nil) != d.wantErr {
				t.Errorf("parseTimestamp(%q) error = %v, wantErr %v", d.input, err, d.wantErr)
			}
			if !gotTime.Equal(d.wantTime) {
				t.Errorf("time = %v, want %v", gotTime, d.wantTime)
			}
			if gotRest != d.wantRest {
				t.Errorf("rest = %q, want %q", gotRest, d.wantRest)
			}
		})
	}
}

func TestParseLogfmt(t *testing.T) {
	data := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{"simple", `a=1 b=2 c=foo`, map[string]string{
			"a": "1",
			"b": "2",
			"c": "foo",
		}},
		{"value with space", `a=1 b="2 3" c=foo`, map[string]string{
			"a": "1",
			"b": "2 3",
			"c": "foo",
		}},
		{"value with =", `a=1 b="2=3" c=foo`, map[string]string{
			"a": "1",
			"b": "2=3",
			"c": "foo",
		}},
		{"empty input", ``, map[string]string{}},
		{"single pair", `a=1`, map[string]string{
			"a": "1",
		}},
		{"quoted value at end", `a=1 msg="hello world"`, map[string]string{
			"a":   "1",
			"msg": "hello world",
		}},
		{"empty quoted value", `a="" b=2`, map[string]string{
			"a": "",
			"b": "2",
		}},
		{"extra whitespace", `  a=1    b=2  `, map[string]string{
			"a": "1",
			"b": "2",
		}},
		{"duplicate key last wins", `a=1 a=2`, map[string]string{
			"a": "2",
		}},
		{"realistic line", `level=error msg="db timeout" code=E42 duration=1.5s`, map[string]string{
			"level":    "error",
			"msg":      "db timeout",
			"code":     "E42",
			"duration": "1.5s",
		}},
		{"empty value at end", `a=`, map[string]string{
			"a": "",
		}},
		{"empty value in middle", `a= b=2`, map[string]string{
			"a": "",
			"b": "2",
		}},
		{"missing key is skipped", `=1`, map[string]string{}},
		{"missing key then pair", `=1 b=2`, map[string]string{
			"b": "2",
		}},
		{"unclosed quote runs to end", `a="unclosed`, map[string]string{
			"a": "unclosed",
		}},
		{"unclosed quote swallows rest", `a="unclosed b=2`, map[string]string{
			"a": "unclosed b=2",
		}},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			result := parseLogfmt(d.input)
			if !maps.Equal(result, d.want) {
				t.Errorf("parseLogfmt(%q) = %q, want %q", d.input, result, d.want)
			}
		})
	}
}

// appEnvelope builds the Envelope that ParseEnvelope would return for an
// app line on web-1 carrying the given payload.
func appEnvelope(payload string) Envelope {
	return Envelope{
		Timestamp: envTime,
		Container: "web-1",
		Payload:   payload,
		Raw:       appLinePrefix + payload,
	}
}

func TestParseAppLog(t *testing.T) {
	payloadTime := time.UnixMilli(1757329978852)

	data := []struct {
		name  string
		input Envelope
		want  LogEntry
	}{
		{"real error log with code", appEnvelope(appLogErrorPayload), LogEntry{
			Timestamp: payloadTime,
			App:       "demo-app",
			Container: "web-1",
			Level:     LevelError,
			Message:   "Error processing request",
			ErrorCode: "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT",
			Raw:       appLinePrefix + appLogErrorPayload,
		}},
		{"info log without error data",
			appEnvelope(`{"app":{"name":"demo-app"},"level":"info","message":"Server started","timestamp":1757329978852}`),
			LogEntry{
				Timestamp: payloadTime,
				App:       "demo-app",
				Container: "web-1",
				Level:     LevelInfo,
				Message:   "Server started",
				Raw:       appLinePrefix + `{"app":{"name":"demo-app"},"level":"info","message":"Server started","timestamp":1757329978852}`,
			}},
		{"unknown level falls back to LevelUnknown",
			appEnvelope(`{"app":{"name":"demo-app"},"level":"fatal","message":"Crash","timestamp":1757329978852}`),
			LogEntry{
				Timestamp: payloadTime,
				App:       "demo-app",
				Container: "web-1",
				Level:     LevelUnknown,
				Message:   "Crash",
				Raw:       appLinePrefix + `{"app":{"name":"demo-app"},"level":"fatal","message":"Crash","timestamp":1757329978852}`,
			}},
		{"missing level falls back to LevelUnknown",
			appEnvelope(`{"app":{"name":"demo-app"},"message":"No level","timestamp":1757329978852}`),
			LogEntry{
				Timestamp: payloadTime,
				App:       "demo-app",
				Container: "web-1",
				Level:     LevelUnknown,
				Message:   "No level",
				Raw:       appLinePrefix + `{"app":{"name":"demo-app"},"message":"No level","timestamp":1757329978852}`,
			}},
		{"missing app and data leave zero values",
			appEnvelope(`{"level":"info","message":"Bare","timestamp":1757329978852}`),
			LogEntry{
				Timestamp: payloadTime,
				Container: "web-1",
				Level:     LevelInfo,
				Message:   "Bare",
				Raw:       appLinePrefix + `{"level":"info","message":"Bare","timestamp":1757329978852}`,
			}},
		{"null data leaves zero error code",
			appEnvelope(`{"app":{"name":"demo-app"},"level":"error","message":"No code","data":null,"timestamp":1757329978852}`),
			LogEntry{
				Timestamp: payloadTime,
				App:       "demo-app",
				Container: "web-1",
				Level:     LevelError,
				Message:   "No code",
				Raw:       appLinePrefix + `{"app":{"name":"demo-app"},"level":"error","message":"No code","data":null,"timestamp":1757329978852}`,
			}},
		// Decision: a wrong-type field is still JSON, so keep every field that
		// decoded instead of throwing the whole entry away.
		{"wrong-type field keeps the fields that decoded",
			appEnvelope(`{"app":{"name":123},"level":"error","message":"Still useful","timestamp":1757329978852}`),
			LogEntry{
				Timestamp: payloadTime,
				Container: "web-1",
				Level:     LevelError,
				Message:   "Still useful",
				Raw:       appLinePrefix + `{"app":{"name":123},"level":"error","message":"Still useful","timestamp":1757329978852}`,
			}},
		{"plain text payload becomes the message",
			appEnvelope("npm WARN deprecated foo@1.0.0"),
			LogEntry{
				Timestamp: envTime,
				Container: "web-1",
				Level:     LevelUnknown,
				Message:   "npm WARN deprecated foo@1.0.0",
				Raw:       appLinePrefix + "npm WARN deprecated foo@1.0.0",
			}},
		{"truncated JSON is treated as plain text",
			appEnvelope(`{"app":{"name":"demo-app"},"level":"err`),
			LogEntry{
				Timestamp: envTime,
				Container: "web-1",
				Level:     LevelUnknown,
				Message:   `{"app":{"name":"demo-app"},"level":"err`,
				Raw:       appLinePrefix + `{"app":{"name":"demo-app"},"level":"err`,
			}},
		{"empty payload", appEnvelope(""), LogEntry{
			Timestamp: envTime,
			Container: "web-1",
			Level:     LevelUnknown,
			Raw:       appLinePrefix,
		}},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			result := ParseAppLog(d.input)
			if !reflect.DeepEqual(result, d.want) {
				t.Errorf("ParseAppLog(%q)\n got: %+v\nwant: %+v", d.input.Payload, result, d.want)
			}
		})
	}
}
