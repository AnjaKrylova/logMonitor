package logs

import (
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
		Timestamp: envTime,
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
		// The one real sample: every field the parser maps is present.
		{"real error log", appLogErrorPayload, LogEntry{
			Timestamp: payloadTime,
			App:       "demo-app",
			Level:     LevelError,
			Message:   "Error processing request",
			ErrorCode: "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT",
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
			LogEntry{Timestamp: envTime, Level: LevelWarn}},
		{"payload bare JSON number",
			`42`,
			LogEntry{Timestamp: envTime, Level: LevelUnknown}},

		// Not JSON: the payload becomes the message, the time comes from the envelope.
		{"plain text payload becomes the message",
			"npm WARN deprecated foo@1.0.0",
			LogEntry{Timestamp: envTime, Level: LevelUnknown, Message: "npm WARN deprecated foo@1.0.0"}},
		{"truncated JSON is treated as plain text",
			`{"level":"err`,
			LogEntry{Timestamp: envTime, Level: LevelUnknown, Message: `{"level":"err`}},
		{"empty payload",
			"",
			LogEntry{Timestamp: envTime, Level: LevelUnknown}}, {
			"2 broken fields", `{"app":{"name":123},"level":"warn","timestamp":"2025-09-08"}`,
			LogEntry{Timestamp: envTime, Level: LevelWarn}},
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
