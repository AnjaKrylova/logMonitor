package logs

import (
	"strconv"
	"strings"
)

func ParseRouterLog(envelope Envelope) LogEntry {
	fields := parseLogfmt(envelope.Payload)
	host := fields["host"]
	delete(fields, "host")

	return LogEntry{
		Timestamp: envelope.Timestamp,
		Container: envelope.Container,
		Level:     levelFromStatus(fields["status"]),
		Host:      host,
		Fields:    fields,
		Raw:       envelope.Raw,
	}
}

// levelFromStatus maps an HTTP status code to a level: 5xx is Error, 4xx is
// Warn, anything else is Info. A missing or non-numeric status is Unknown.
func levelFromStatus(status string) LogLevel {
	code, err := strconv.Atoi(status)
	if err != nil {
		return LevelUnknown
	}

	switch {
	case code >= 500:
		return LevelError
	case code >= 400:
		return LevelWarn
	default:
		return LevelInfo
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
