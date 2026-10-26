package logs

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Envelope struct {
	Timestamp time.Time
	Container string
	Payload   string
	Raw       string
}

// ParseEnvelope splits a Scalingo line "date clock offset ZONE[container] payload"
// into its parts. The zone is skipped and may be followed by a space
// ("CEST[web-1]", "utc [worker-1]").
//
// A line that does not match this shape returns an envelope with only Raw set,
// together with an error, so callers can still store it.
func ParseEnvelope(line string) (Envelope, error) {
	fallback := Envelope{Raw: line}

	t, rest, err := parseTimestamp(line)
	if err != nil {
		return fallback, err
	}

	zone, rest, found := strings.Cut(rest, "[")
	if !found {
		return fallback, errors.New("missing [container]")
	}
	// The zone is a single word; a space inside means the "[" belongs to
	// the payload of a line without a container.
	if strings.Contains(strings.TrimSpace(zone), " ") {
		return fallback, errors.New("missing [container]")
	}

	container, payload, found := strings.Cut(rest, "]")
	if !found {
		return fallback, errors.New("missing ] after container")
	}

	return Envelope{Timestamp: t, Container: container, Payload: strings.TrimSpace(payload), Raw: line}, nil
}

// parseTimestamp reads "date clock offset" from the start of a line and
// returns everything after the offset as rest.
func parseTimestamp(line string) (time.Time, string, error) {
	date, rest, _ := strings.Cut(line, " ")
	clock, rest, _ := strings.Cut(rest, " ")
	offset, rest, _ := strings.Cut(rest, " ")

	parsedTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700", date+" "+clock+" "+offset)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("parse timestamp: %w", err)
	}

	return parsedTime, rest, nil
}
