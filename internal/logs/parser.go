package logs

import (
	"encoding/json"
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

func ParseEnvelope(line string) (Envelope, error) {
	fallback := Envelope{Payload: strings.TrimSpace(line), Raw: line}

	date, rest, foundDate := strings.Cut(line, " ")
	clock, rest, foundClock := strings.Cut(rest, " ")
	offset, rest, _ := strings.Cut(rest, " ")
	if !foundDate || !foundClock {
		return fallback, errors.New("parse envelope: no timestamp found")
	}
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700", date+" "+clock+" "+offset)
	if err != nil {
		return fallback, fmt.Errorf("parse envelope: %w", err)
	}

	if word, after, _ := strings.Cut(rest, " "); isZoneAbbreviation(word) {
		rest = after
	}

	var container string
	if strings.HasPrefix(rest, "[") {
		if end := strings.IndexByte(rest, ']'); end != -1 {
			container = rest[1:end]
			rest = rest[end+1:]
		}
	}

	return Envelope{
		Timestamp: t,
		Container: container,
		Payload:   strings.TrimSpace(rest),
		Raw:       line,
	}, nil
}

// isZoneAbbreviation reports whether s looks like a time zone abbreviation
// such as "UTC" or "CEST".
func isZoneAbbreviation(s string) bool {
	if len(s) < 3 || len(s) > 5 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

type AppLogPayload struct {
	App struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"app"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"` // Unix epoch milliseconds
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

func ParseAppLog(envelope Envelope) (LogEntry, error) {
	var parsedAppLog AppLogPayload
	err := json.Unmarshal([]byte(envelope.Payload), &parsedAppLog)
	if err != nil {
		return LogEntry{}, fmt.Errorf("parse app log: %w", err)
	}

	level, err := ParseLogLevel(parsedAppLog.Level)
	if err != nil {
		return LogEntry{}, fmt.Errorf("parse app log: %w", err)
	}

	return LogEntry{
		Timestamp: time.UnixMilli(parsedAppLog.Timestamp),
		App:       parsedAppLog.App.Name,
		Container: envelope.Container,
		Level:     level,
		Message:   parsedAppLog.Message,
		ErrorCode: parsedAppLog.Data.Error.Code,
	}, nil
}

// parseLogfmt parses space-separated key=value pairs, where values may be
// double-quoted. It never fails: malformed input is parsed on a best-effort
// basis, and pairs without a key are skipped.
func parseLogfmt(s string) map[string]string {
	logMap := make(map[string]string)
	rest := s
	for rest != "" {
		rawKey, after, found := strings.Cut(rest, "=")
		if !found {
			break
		}
		rest = after
		key := strings.TrimSpace(rawKey)
		var value string
		if strings.HasPrefix(rest, "\"") {
			rest = rest[1:]
			// A missing closing quote makes the value run to the end of the line.
			value, rest, _ = strings.Cut(rest, "\"")
		} else {
			value, rest, _ = strings.Cut(rest, " ")
		}
		if key != "" {
			logMap[key] = value
		}
	}
	return logMap
}
