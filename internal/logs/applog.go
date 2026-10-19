package logs

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// mappedKeys are payload keys that already have a LogEntry field; they are kept out of Fields.
var mappedKeys = []string{"app.name", "data.error.code", "level", "message", "timestamp"}

type appLogPayload struct {
	App struct {
		Name string `json:"name"`
	} `json:"app"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"` // Unix epoch milliseconds
	Data      struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	} `json:"data"`
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

	var raw map[string]any
	_ = json.Unmarshal([]byte(envelope.Payload), &raw) // non-object JSON (e.g. 42) leaves raw nil → no Fields

	out := map[string]string{}
	flatten("", raw, out)
	for _, key := range mappedKeys {
		delete(out, key)
	}
	if len(out) == 0 {
		out = nil
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
		Fields:    out,
	}
}

// flatten writes every leaf value of v into out as a string, keyed by its
// dotted path from prefix (origin.file, params.0). null values and empty
// objects or arrays add no key.
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
