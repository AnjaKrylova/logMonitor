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
	Level         string `json:"level"`
	Message       string `json:"message"`
	MessageParams []any  `json:"params"`
	Timestamp     int64  `json:"timestamp"` // Unix epoch milliseconds
	Data          struct {
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

	timestamp := envelope.Timestamp
	if parsedAppLog.Timestamp != 0 {
		timestamp = time.UnixMilli(parsedAppLog.Timestamp)
	}

	level, _ := ParseLogLevel(parsedAppLog.Level)

	return LogEntry{
		Timestamp:     timestamp,
		App:           parsedAppLog.App.Name,
		Container:     envelope.Container,
		Level:         level,
		Message:       parsedAppLog.Message,
		MessageParams: paramsToStrings(parsedAppLog.MessageParams),
		ErrorCode:     parsedAppLog.Data.Error.Code,
		Fields:        extraFields(envelope.Payload),
		Raw:           envelope.Raw,
	}
}

// extraFields returns every payload key that has no LogEntry field of its own, flattened.
func extraFields(payload string) map[string]string {
	var raw map[string]any
	_ = json.Unmarshal([]byte(payload), &raw) // non-object JSON (e.g. 42) leaves raw nil → no Fields

	out := map[string]string{}
	// params is a list, so it is removed before flatten splits it into params.0, params.1, …;
	// mappedKeys are single values and can be removed after.
	delete(raw, "params")
	flatten("", raw, out)
	for _, key := range mappedKeys {
		delete(out, key)
	}
	if len(out) == 0 {
		out = nil
	}
	return out
}

// flatten writes every leaf value of v into out as a string, keyed by its
// dotted path from prefix (origin.file, params.0). null values and empty
// objects or arrays add no key. Objects and arrays recurse one level deeper.
//
// Example:
//
//	{"origin":{"line":158},"params":["a",3],"ok":true,"extra":null}
//
// becomes
//
//	"origin.line": "158", "params.0": "a", "params.1": "3", "ok": "true"
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

func paramsToStrings(params []any) []string {
	var result []string
	for _, v := range params {
		switch x := v.(type) {
		case string:
			result = append(result, x)
		case float64:
			result = append(result, strconv.FormatFloat(x, 'f', -1, 64))
		case bool:
			result = append(result, strconv.FormatBool(x))
		}
	}
	return result
}
