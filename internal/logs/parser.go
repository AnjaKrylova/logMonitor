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
	openBracket := strings.IndexByte(line, '[')
	closeBracket := strings.IndexByte(line, ']')
	if openBracket == -1 {
		return Envelope{}, errors.New("openBracket is not found")
	}
	if closeBracket == -1 {
		return Envelope{}, errors.New("closeBracket is not found")
	}
	if closeBracket < openBracket {
		return Envelope{}, errors.New("Brackets are in wrong order")
	}
	rawTime := strings.TrimSpace(line[:openBracket])
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", rawTime)
	if err != nil {
		return Envelope{}, fmt.Errorf("parse timestamp: %w", err)
	}
	container := line[openBracket+1 : closeBracket]
	payload := strings.TrimSpace(line[closeBracket+1:])
	return Envelope{Timestamp: t, Container: container, Payload: payload}, nil
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
	if err != nil {
		return LogEntry{
			Container: envelope.Container,
			Level:     LevelUnknown,
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
