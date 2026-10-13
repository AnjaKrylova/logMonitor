package logs

import (
	"encoding/json"
	"errors"
	"strconv"
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

func flatten(path string, v any, out map[string]string) {
	switch x := v.(type) {
	case string:
		out[path] = x
	case float64:
		out[path] = strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		out[path] = strconv.FormatBool(x)
	case nil:
		// null adds no key
	case map[string]any:
		for k, val := range x {
			flatten(joinKey(path, k), val, out)
		}
	case []any:
		for i, val := range x {
			flatten(joinKey(path, strconv.Itoa(i)), val, out)
		}
	}
}

func joinKey(prefix, k string) string {
	if prefix == "" {
		return k
	}
	return prefix + "." + k
}
