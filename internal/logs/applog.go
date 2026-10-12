package logs

import (
	"encoding/json"
	"errors"
	"time"
)

type appLogPayload struct {
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
	var parsedAppLog appLogPayload
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

	timestamp := envelope.Timestamp

	if parsedAppLog.Timestamp != 0 {
		timestamp = time.UnixMilli(parsedAppLog.Timestamp)
	}
	level, _ := ParseLogLevel(parsedAppLog.Level)

	return LogEntry{
		Timestamp: timestamp,
		App:       parsedAppLog.App.Name,
		Container: envelope.Container,
		Level:     level,
		Message:   parsedAppLog.Message,
		ErrorCode: parsedAppLog.Data.Error.Code,
		Raw:       envelope.Raw,
	}
}
