package logs

import (
	"testing"
	"time"
)

// Timestamp at the start of every Scalingo line, without and with the zone.
// Real lines glue the container to the zone: "... +0200 CEST[web-1] ...".
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
		{"no zone abbreviation", timeNoZone + " [web-1] hello", Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   "hello",
		}, false},
		{"no container", timePrefix + " some text", Envelope{
			Timestamp: timePrefixParsed,
			Payload:   "some text",
		}, false},
		{"no container, payload has brackets", timePrefix + ` {"params":["a"]}`, Envelope{
			Timestamp: timePrefixParsed,
			Payload:   `{"params":["a"]}`,
		}, false},
		{"timestamp only", timePrefix, Envelope{
			Timestamp: timePrefixParsed,
		}, false},
		{"no space after container", timePrefix + "[web-1]hello", Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   "hello",
		}, false},
		// Without a parsable timestamp nothing in the line is trusted:
		// ParseEnvelope returns an envelope with only Raw set, together with
		// an error. Real Scalingo lines always carry a timestamp, so this is
		// a safety net, not a path that needs structured parsing.
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
		{"zone glued to container", timePrefix + "[web-1] hello", timePrefixParsed, "[web-1] hello", false},
		{"no zone abbreviation", timeNoZone + " [web-1] hello", timePrefixParsed, "[web-1] hello", false},
		{"timestamp with zone only", timePrefix, timePrefixParsed, "", false},
		{"timestamp without zone only", timeNoZone, timePrefixParsed, "", false},
		{"UTC zone", "2025-09-08 11:12:58.867176204 +0000 UTC hello", timePrefixParsed, "hello", false},
		{"fewer fractional digits", "2025-09-08 13:12:58.8 +0200 CEST x",
			time.Date(2025, 9, 8, 11, 12, 58, 800000000, time.UTC), "x", false},
		{"no fractional seconds", "2025-09-08 13:12:58 +0200 CEST x",
			time.Date(2025, 9, 8, 11, 12, 58, 0, time.UTC), "x", false},
		{"brackets in payload are not a container", timePrefix + ` {"params":["a"]}`, timePrefixParsed, `{"params":["a"]}`, false},

		// The zone word is skipped only when it looks like one: 3-5 uppercase
		// letters, standing alone or directly followed by "[container]".
		{"text glued to zone is not a zone", timePrefix + "abc", timePrefixParsed, "CESTabc", false},
		{"lowercase word is not a zone", timeNoZone + " hello world", timePrefixParsed, "hello world", false},
		{"long uppercase word is not a zone", timeNoZone + " WARNING disk full", timePrefixParsed, "WARNING disk full", false},
		// Accepted trade-off: a short uppercase word right after the offset is
		// indistinguishable from a zone and is skipped. Raw still keeps it.
		{"short uppercase word is taken as zone", timeNoZone + " WARN disk full", timePrefixParsed, "disk full", false},
		{"lowercase short word is not a zone", timeNoZone + " info disk full", timePrefixParsed, "info disk full", false},

		{"empty line", ``, time.Time{}, "", true},
		{"no timestamp", `[web-1] hello`, time.Time{}, "", true},
		{"word instead of date", `yesterday [web-1] hello`, time.Time{}, "", true},
		{"date without clock", `2025-09-08 [web-1] hello`, time.Time{}, "", true},
		{"missing offset", "2025-09-08 13:12:58.867176204", time.Time{}, "", true},
		{"zone instead of offset", "2025-09-08 13:12:58.867176204 CEST [web-1] hello", time.Time{}, "", true},
		{"impossible date", "2025-13-45 13:12:58 +0200 CEST x", time.Time{}, "", true},
		{"leading whitespace", "  " + timePrefix + " x", time.Time{}, "", true},
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
