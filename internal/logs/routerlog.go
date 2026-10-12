package logs

import (
	"strconv"
	"strings"
)

func ParseRouterLog(envelope Envelope) LogEntry {
	fields := parseLogfmt(envelope.Payload)
	level := LevelUnknown
	value := fields["status"]
	code, err := strconv.Atoi(value)
	if err == nil {
		switch {
		case code >= 500:
			level = LevelError
		case code >= 400:
			level = LevelWarn
		default:
			level = LevelInfo
		}
	}
	return LogEntry{
		Timestamp: envelope.Timestamp,
		Container: envelope.Container,
		Level:     level,
		Fields:    fields,
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
			// A missing closing quote makes the value run sto the end of the line.
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
