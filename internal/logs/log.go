package logs

import (
	"fmt"
	"strings"
	"time"
)

type LogLevel string

const (
	LevelDebug   LogLevel = "DEBUG"
	LevelInfo    LogLevel = "INFO"
	LevelWarn    LogLevel = "WARN"
	LevelError   LogLevel = "ERROR"
	LevelUnknown LogLevel = "UNKNOWN"
)

type LogEntry struct {
	ID            string
	Timestamp     time.Time
	App           string
	Container     string // e.g. "router", "web-1"
	Level         LogLevel
	Host          string
	Message       string
	MessageParams []string
	ErrorCode     string // e.g. "FETCH_CLIENT_ERROR_REQUEST_TIMEOUT"
	Fields        map[string]string
	Raw           string
}

func ParseLogLevel(s string) (LogLevel, error) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return LevelDebug, nil
	case "INFO":
		return LevelInfo, nil
	case "WARN":
		return LevelWarn, nil
	case "ERROR":
		return LevelError, nil
	default:
		return LevelUnknown, fmt.Errorf("unknown log level: %q", s)
	}
}

func (l LogEntry) IsError() bool {
	return l.Level == LevelError
}

// ContainerType returns Container without its index: "web-1" → "web", "one-off-1234" → "one-off".
func (l LogEntry) ContainerType() string {
	i := strings.LastIndex(l.Container, "-")
	if i == -1 {
		return l.Container
	}
	return l.Container[:i]
}
