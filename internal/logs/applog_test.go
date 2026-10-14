package logs

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

var payloadTime = time.UnixMilli(1757329978852)

// Prefix of a real Scalingo line for container web-1.
const appLinePrefix = timePrefix + " [web-1] "

// Real sample from a Scalingo app container (stack trace shortened).
const appLogErrorPayload = `{"app":{"name":"demo-app","version":"3.4.2"},"data":{"error":{"code":"FETCH_CLIENT_ERROR_REQUEST_TIMEOUT","message":"Request aborted after 5000 milliseconds","stack":"Error: Request aborted after 5000 milliseconds\n at ClientRequest.parseResponseError (/app/node_modules/@hedia/fetch/src/index.ts:405:10)"}},"level":"error","message":"Error processing request","origin":{"column":30,"file":"src/main.ts","line":158},"params":[],"timestamp":1757329978852}`

// appEnvelope builds the Envelope that ParseEnvelope would return for an
// app line on web-1 carrying the given payload.
func appEnvelope(payload string) Envelope {
	return Envelope{
		Timestamp: timePrefixParsed,
		Container: "web-1",
		Payload:   payload,
		Raw:       appLinePrefix + payload,
	}
}

func TestParseAppLog(t *testing.T) {
	// Each row gives only the payload; the loop wraps it with appEnvelope.
	// Container and Raw are passed through from the envelope unchanged in
	// every case, so the loop fills them into want instead of each row.
	data := []struct {
		name    string
		payload string
		want    LogEntry
	}{
		{"real error log", appLogErrorPayload, LogEntry{
			Timestamp: payloadTime,
			App:       "demo-app",
			Level:     LevelError,
			Message:   "Error processing request",
			ErrorCode: "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT",
			Fields: map[string]string{
				"app.version":        "3.4.2",
				"data.error.message": "Request aborted after 5000 milliseconds",
				"data.error.stack":   "Error: Request aborted after 5000 milliseconds\n at ClientRequest.parseResponseError (/app/node_modules/@hedia/fetch/src/index.ts:405:10)",
				"origin.column":      "30",
				"origin.file":        "src/main.ts",
				"origin.line":        "158",
			},
		}},

		// JSON payloads: each one carries only the fields its row is about.
		{"info log without app or error data",
			`{"level":"info","message":"Server started","timestamp":1757329978852}`,
			LogEntry{Timestamp: payloadTime, Level: LevelInfo, Message: "Server started"}},
		{"unknown level falls back to LevelUnknown",
			`{"level":"fatal","timestamp":1757329978852}`,
			LogEntry{Timestamp: payloadTime, Level: LevelUnknown}},
		{"missing level falls back to LevelUnknown",
			`{"message":"No level","timestamp":1757329978852}`,
			LogEntry{Timestamp: payloadTime, Level: LevelUnknown, Message: "No level"}},
		{"null data leaves zero error code",
			`{"level":"error","data":null,"timestamp":1757329978852}`,
			LogEntry{Timestamp: payloadTime, Level: LevelError}},
		// Decision: a wrong-type field is still JSON, so keep every field that
		// decoded instead of throwing the whole entry away.
		{"wrong-type field keeps the fields that decoded",
			`{"app":{"name":123},"level":"error","message":"Still useful","timestamp":1757329978852}`,
			LogEntry{Timestamp: payloadTime, Level: LevelError, Message: "Still useful"}},
		{"wrong-type timestamp falls back to envelope time",
			`{"level":"warn","timestamp":"2025-09-08"}`,
			LogEntry{Timestamp: timePrefixParsed, Level: LevelWarn}},
		{"payload bare JSON number",
			`42`,
			LogEntry{Timestamp: timePrefixParsed, Level: LevelUnknown}},

		// Not JSON: the payload becomes the message, the time comes from the envelope.
		{"plain text payload becomes the message",
			"npm WARN deprecated foo@1.0.0",
			LogEntry{Timestamp: timePrefixParsed, Level: LevelUnknown, Message: "npm WARN deprecated foo@1.0.0"}},
		{"truncated JSON is treated as plain text",
			`{"level":"err`,
			LogEntry{Timestamp: timePrefixParsed, Level: LevelUnknown, Message: `{"level":"err`}},
		{"empty payload",
			"",
			LogEntry{Timestamp: timePrefixParsed, Level: LevelUnknown}}, {
			"2 broken fields", `{"app":{"name":123},"level":"warn","timestamp":"2025-09-08"}`,
			LogEntry{Timestamp: timePrefixParsed, Level: LevelWarn}},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			input := appEnvelope(d.payload)
			want := d.want
			want.Container = input.Container
			want.Raw = input.Raw

			result := ParseAppLog(input)
			if !reflect.DeepEqual(result, want) {
				t.Errorf("ParseAppLog(%q)\n got: %+v\nwant: %+v", d.payload, result, want)
			}
		})
	}
}

func TestFlatten(t *testing.T) {
	data := []struct {
		name  string
		input string
		want  map[string]string
	}{{
		"simple string", `{"k":"v"}`, map[string]string{"k": "v"},
	}, {
		"simple number", `{"line":158}`, map[string]string{"line": "158"},
	}, {
		"big number", `{"big":1757329978852}`, map[string]string{"big": "1757329978852"},
	}, {
		"boolean", `{"ok":true}`, map[string]string{"ok": "true"},
	}, {
		"nested object", `{"origin":{"file":"src/main.ts","line":158}}`, map[string]string{
			"origin.file": "src/main.ts",
			"origin.line": "158",
		},
	}, {
		"array of primitives", `{"params":["a",3]}`, map[string]string{
			"params.0": "a",
			"params.1": "3",
		},
	}, {
		"array of objects", `{"params":[{"id":1}]}`, map[string]string{
			"params.0.id": "1",
		},
	}, {
		"null value", `{"k":null}`, map[string]string{},
	}, {
		"empty values", `{"a":null,"b":[],"c":{}}`, map[string]string{},
	}, {
		"mixed null and non-null", `{"a":null,"b":"x"}`, map[string]string{
			"b": "x",
		},
	}, {
		"empty string value", `{"k":""}`, map[string]string{
			"k": "",
		},
	}, {
		"float value", `{"ratio":1.5}`, map[string]string{
			"ratio": "1.5",
		},
	}}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var v any
			err := json.Unmarshal([]byte(d.input), &v)
			if err != nil {
				t.Fatalf("Unmarshal(%q) failed: %v", d.input, err)
			}

			out := map[string]string{}
			flatten("", v, out)
			if !reflect.DeepEqual(out, d.want) {
				t.Errorf("flatten(%q)\n got: %+v\nwant: %+v", d.input, out, d.want)
			}
		})
	}
}
