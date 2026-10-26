package logs

import (
	"testing"
	"time"
)

// Real Scalingo lines: "date clock offset ZONE[container] payload", usually
// with the container glued to the zone: "... +0200 CEST[web-1] ...".
const (
	timeNoZone = "2025-09-08 13:12:58.867176204 +0200"
	timePrefix = timeNoZone + " CEST"
)

// 2025-09-08 13:12:58.867176204 +0200 CEST as an instant.
var timePrefixParsed = time.Date(2025, 9, 8, 11, 12, 58, 867176204, time.UTC)

func TestParseEnvelope(t *testing.T) {
	const routerPayload = `method=GET path="/password" status=200 duration=0.030s`
	const bracketsPayload = `{"params":["a","b"],"message":"see [docs]"}`

	// Raw is always the input line, so rows leave it out; the loop sets it.
	data := []struct {
		name    string
		input   string
		want    Envelope
		wantErr bool
	}{
		{"app line", appLinePrefix + appLogErrorPayload, Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   appLogErrorPayload,
		}, false},
		{"router line", routerLinePrefix + routerPayload, Envelope{
			Timestamp: timePrefixParsed,
			Container: "router",
			Payload:   routerPayload,
		}, false},
		{"winter time (CET)", "2026-10-26 10:00:00.123456789 +0100 CET[web-1] hello", Envelope{
			Timestamp: time.Date(2026, 10, 26, 9, 0, 0, 123456789, time.UTC),
			Container: "web-1",
			Payload:   "hello",
		}, false},
		{"space before container, lowercase zone", "2026-10-25 15:49:50.553980154 +0000 utc [worker-1] /app/.env not found", Envelope{
			Timestamp: time.Date(2026, 10, 25, 15, 49, 50, 553980154, time.UTC),
			Container: "worker-1",
			Payload:   "/app/.env not found",
		}, false},
		{"brackets inside payload belong to payload", appLinePrefix + bracketsPayload, Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   bracketsPayload,
		}, false},
		{"payload whitespace is trimmed", appLinePrefix + "  hello  ", Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   "hello",
		}, false},
		{"empty payload", appLinePrefix, Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
		}, false},

		// A line that does not match the real format keeps only Raw and
		// comes back with an error; callers still store it (nothing is lost).
		{"no container", timePrefix + " some text", Envelope{}, true},
		{"no container, payload has brackets", timePrefix + " error: see [docs]", Envelope{}, true},
		{"missing ] after container", timePrefix + "[web-1 hello", Envelope{}, true},
		{"timestamp only", timePrefix, Envelope{}, true},
		{"split fractional seconds (seen after a crash)", "2026-10-25 15:49:50.55398 0154 +0000 utc [worker-1] x", Envelope{}, true},
		{"empty line", ``, Envelope{}, true},
		{"bad timestamp", `yesterday [web-1] hello`, Envelope{}, true},
		{"date without time", `2025-09-08 [web-1] hello`, Envelope{}, true},
		{"no timestamp at all", `[web-1] hello`, Envelope{}, true},
		{"JSON only", `{"level":"error","message":"boom"}`, Envelope{}, true},
		{"Raw keeps surrounding whitespace", "  plain text  ", Envelope{}, true},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			want := d.want
			want.Raw = d.input

			result, err := ParseEnvelope(d.input)
			if (err != nil) != d.wantErr {
				t.Errorf("ParseEnvelope(%q) error = %v, wantErr %v", d.input, err, d.wantErr)
			}

			// The envelope is checked even on error: Raw must never be lost.
			// time.Parse gives the CEST timestamp a synthetic location, so compare
			// instants with Equal instead of comparing whole structs.
			if !result.Timestamp.Equal(want.Timestamp) {
				t.Errorf("Timestamp = %v, want %v", result.Timestamp, want.Timestamp)
			}
			if result.Container != want.Container {
				t.Errorf("Container = %q, want %q", result.Container, want.Container)
			}
			if result.Payload != want.Payload {
				t.Errorf("Payload = %q, want %q", result.Payload, want.Payload)
			}
			if result.Raw != want.Raw {
				t.Errorf("Raw = %q, want %q", result.Raw, want.Raw)
			}
		})
	}
}

// parseTimestamp reads "date clock offset" from the start of a line.
// rest is everything after the offset (zone, container and payload), with
// the single separating space removed. On error, t is zero and rest is "".
func TestParseTimestamp(t *testing.T) {
	data := []struct {
		name     string
		input    string
		wantTime time.Time
		wantRest string
		wantErr  bool
	}{
		{"real prefix", timePrefix + "[web-1] hello", timePrefixParsed, "CEST[web-1] hello", false},
		{"winter time (CET)", "2026-10-26 10:00:00.123456789 +0100 CET[web-1] x",
			time.Date(2026, 10, 26, 9, 0, 0, 123456789, time.UTC), "CET[web-1] x", false},
		{"timestamp only", timeNoZone, timePrefixParsed, "", false},
		{"fewer fractional digits", "2025-09-08 13:12:58.8 +0200 CEST[web-1] x",
			time.Date(2025, 9, 8, 11, 12, 58, 800000000, time.UTC), "CEST[web-1] x", false},
		{"no fractional seconds", "2025-09-08 13:12:58 +0200 CEST[web-1] x",
			time.Date(2025, 9, 8, 11, 12, 58, 0, time.UTC), "CEST[web-1] x", false},

		{"empty line", ``, time.Time{}, "", true},
		{"no timestamp", `[web-1] hello`, time.Time{}, "", true},
		{"word instead of date", `yesterday [web-1] hello`, time.Time{}, "", true},
		{"date without clock", `2025-09-08 [web-1] hello`, time.Time{}, "", true},
		{"missing offset", "2025-09-08 13:12:58.867176204", time.Time{}, "", true},
		{"zone instead of offset", "2025-09-08 13:12:58.867176204 CEST[web-1] hello", time.Time{}, "", true},
		{"impossible date", "2025-13-45 13:12:58 +0200 CEST[web-1] x", time.Time{}, "", true},
		{"leading whitespace", "  " + timePrefix + "[web-1] x", time.Time{}, "", true},
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
