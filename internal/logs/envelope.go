package logs

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

type Envelope struct {
	Timestamp time.Time
	Container string
	Payload   string
	Raw       string
}

func ParseEnvelope(line string) (Envelope, error) {
	fallbackEnvelope := Envelope{
		Raw: line,
	}

	t, rest, err := parseTimestamp(line)
	if err != nil {
		return fallbackEnvelope, fmt.Errorf("parse timestamp: %w", err)
	}

	var container string
	var payload string
	if strings.HasPrefix(rest, "[") {
		closeBracketIndex := strings.Index(rest, "]")
		if closeBracketIndex != -1 {
			container = rest[1:closeBracketIndex]
			payload = strings.TrimSpace(rest[closeBracketIndex+1:])
		}
	} else {
		payload = strings.TrimSpace(rest)
	}

	return Envelope{Timestamp: t, Container: container, Payload: payload, Raw: line}, nil
}

func parseTimestamp(line string) (time.Time, string, error) {
	date, rest, _ := strings.Cut(line, " ")
	clock, rest, _ := strings.Cut(rest, " ")
	offset, rest, _ := strings.Cut(rest, " ")
	wordContainer, after, _ := strings.Cut(rest, " ")
	word, container, _ := strings.Cut(wordContainer, "[")
	// The zone word is optional; when present it is always cut off.
	if isTimeZone(word) {
		if container == "" {
			// "CEST hello": zone and the separating space go.
			rest = after
		} else {
			// "CEST[web-1] hello" (real Scalingo format): only the zone goes,
			// so rest starts with "[container]" for ParseEnvelope.
			rest = strings.TrimPrefix(rest, word)
		}
	}

	rawTime := strings.Join([]string{date, clock, offset}, " ")
	parsedTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700", rawTime)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("parse timestamp: %w", err)
	}

	return parsedTime, rest, nil
}

func isTimeZone(s string) bool {
	if !(len(s) >= 3 && len(s) <= 5) {
		return false
	}

	for _, r := range s {
		if !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}
