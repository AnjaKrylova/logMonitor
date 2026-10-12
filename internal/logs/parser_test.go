package logs

import (
	"reflect"
	"testing"
	"time"
)

var receivedAt = time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

func TestParseLog(t *testing.T) {

	data := []struct {
		name string
		line string
		want LogEntry
	}{
		{"router line",
			routerLinePrefix + "status=200", LogEntry{
				Timestamp: timePrefixParsed,
				Container: "router",
				Level:     LevelInfo,
				Fields:    map[string]string{"status": "200"},
			},
		},
		{"app line",
			appLinePrefix + `{"level":"error","message":"boom","timestamp":1757329978852}`,
			LogEntry{
				Timestamp: payloadTime.UTC(),
				Container: "web-1",
				Level:     LevelError,
				Message:   "boom",
			}},
		{"line without timestamp",
			"just some text",
			LogEntry{
				Timestamp: receivedAt,
				Level:     LevelUnknown,
			},
		},
		{"empty line",
			"",
			LogEntry{
				Timestamp: receivedAt,
				Level:     LevelUnknown,
			}}}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			want := d.want
			want.Raw = d.line

			result := ParseLog(d.line, receivedAt)
			if result.Timestamp.Location() != time.UTC {
				t.Errorf("Timestamp location = %v, want UTC", result.Timestamp.Location())
			}
			if !reflect.DeepEqual(result, want) {
				t.Errorf("ParseLog(%q)\n got: %+v\nwant: %+v", d.line, result, want)
			}
		})
	}
}
