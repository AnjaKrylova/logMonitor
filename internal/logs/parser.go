package logs

import (
	"encoding/json"
	"errors"
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

func ParseAppLog(envelope Envelope) LogEntry {
	var parsedAppLog AppLogPayload
	err := json.Unmarshal([]byte(envelope.Payload), &parsedAppLog)
	_, ok := errors.AsType[*json.SyntaxError](err)
	if ok {
		return LogEntry{
			Timestamp: envelope.Timestamp,
			Container: envelope.Container,
			Level:     LevelUnknown,
			Message:   envelope.Payload,
			Raw:       envelope.Raw,
		}
	}

	level, _ := ParseLogLevel(parsedAppLog.Level)

	return LogEntry{
		Timestamp: time.UnixMilli(parsedAppLog.Timestamp),
		App:       parsedAppLog.App.Name,
		Container: envelope.Container,
		Level:     level,
		Message:   parsedAppLog.Message,
		ErrorCode: parsedAppLog.Data.Error.Code,
		Raw:       envelope.Raw,
	}
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

func parseTimestamp(line string) (time.Time, string, error) {
	date, rest, _ := strings.Cut(line, " ")
	clock, rest, _ := strings.Cut(rest, " ")
	offset, rest, _ := strings.Cut(rest, " ")
	word, after, _ := strings.Cut(rest, " ")
	if isTimeZone(word) {
		rest = after
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
