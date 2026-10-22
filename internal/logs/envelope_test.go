package logs

import (
	"testing"
	"time"
)

// Timestamp at the start of every Scalingo line.
const timePrefix = "2025-09-08 13:12:58.867176204 +0200 CEST"

// 2025-09-08 13:12:58.867176204 +0200 CEST as an instant.
var timePrefixParsed = time.Date(2025, 9, 8, 11, 12, 58, 867176204, time.UTC)

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
			Timestamp: timePrefixParsed,
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
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   `{"params":["a","b"],"message":"see [docs]"}`,
			Raw:       payloadWithBrackets,
		}, false},
		{"payload whitespace is trimmed", appLinePrefix + "  hello  ", Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   "hello",
			Raw:       appLinePrefix + "  hello  ",
		}, false},
		{"empty payload", appLinePrefix, Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Raw:       appLinePrefix,
		}, false},
		{"no zone abbreviation", "2025-09-08 13:12:58.867176204 +0200 [web-1] hello", Envelope{
			Timestamp: timePrefixParsed,
			Container: "web-1",
			Payload:   "hello",
			Raw:       "2025-09-08 13:12:58.867176204 +0200 [web-1] hello",
		}, false},
		{"no container", timePrefix + " some text", Envelope{
			Timestamp: timePrefixParsed,
			Payload:   "some text",
			Raw:       timePrefix + " some text",
		}, false},
		{"no container, payload has brackets", timePrefix + ` {"params":["a"]}`, Envelope{
			Timestamp: timePrefixParsed,
			Payload:   `{"params":["a"]}`,
			Raw:       timePrefix + ` {"params":["a"]}`,
		}, false},
		{"timestamp only", timePrefix, Envelope{
			Timestamp: timePrefixParsed,
			Raw:       timePrefix,
		}, false},
		{"no space after container", timePrefix + " [web-1]hello", Envelope{
			Timestamp: timePrefixParsed,
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
		{"full prefix with zone and container", timePrefix + " [web-1] hello", timePrefixParsed, "[web-1] hello", false},
		{"no zone abbreviation", "2025-09-08 13:12:58.867176204 +0200 [web-1] hello", timePrefixParsed, "[web-1] hello", false},
		{"timestamp with zone only", timePrefix, timePrefixParsed, "", false},
		{"timestamp without zone only", "2025-09-08 13:12:58.867176204 +0200", timePrefixParsed, "", false},
		{"UTC zone", "2025-09-08 11:12:58.867176204 +0000 UTC hello", timePrefixParsed, "hello", false},
		{"fewer fractional digits", "2025-09-08 13:12:58.8 +0200 CEST x",
			time.Date(2025, 9, 8, 11, 12, 58, 800000000, time.UTC), "x", false},
		{"no fractional seconds", "2025-09-08 13:12:58 +0200 CEST x",
			time.Date(2025, 9, 8, 11, 12, 58, 0, time.UTC), "x", false},
		{"payload brackets are not touched", timePrefix + ` {"params":["a"]}`, timePrefixParsed, `{"params":["a"]}`, false},

		// The zone word is skipped only when it looks like one: 3-5 uppercase
		// letters, standing alone.
		{"text glued to zone is not a zone", timePrefix + "abc", timePrefixParsed, "CESTabc", false},
		{"lowercase word is not a zone", "2025-09-08 13:12:58.867176204 +0200 hello world", timePrefixParsed, "hello world", false},
		{"long uppercase word is not a zone", "2025-09-08 13:12:58.867176204 +0200 WARNING disk full", timePrefixParsed, "WARNING disk full", false},
		// Accepted trade-off: a short uppercase word right after the offset is
		// indistinguishable from a zone and is skipped. Raw still keeps it.
		{"short uppercase word is taken as zone", "2025-09-08 13:12:58.867176204 +0200 WARN disk full", timePrefixParsed, "disk full", false},

		{"empty line", ``, time.Time{}, "", true},
		{"no timestamp", `[web-1] hello`, time.Time{}, "", true},
		{"word instead of date", `yesterday [web-1] hello`, time.Time{}, "", true},
		{"date without clock", `2025-09-08 [web-1] hello`, time.Time{}, "", true},
		{"missing offset", "2025-09-08 13:12:58.867176204", time.Time{}, "", true},
		{"zone instead of offset", "2025-09-08 13:12:58.867176204 CEST [web-1] hello", time.Time{}, "", true},
		{"impossible date", "2025-13-45 13:12:58 +0200 CEST x", time.Time{}, "", true},
		{"leading whitespace", "  " + timePrefix + " x", time.Time{}, "", true},
		{"lowercase short word is not a zone", "2025-09-08 13:12:58.867176204 +0200 info disk full", timePrefixParsed, "info disk full", false},
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
