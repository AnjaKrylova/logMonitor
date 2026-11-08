package logs

import (
	"testing"
)

func TestParseLogLevel(t *testing.T) {
	data := []struct {
		name    string
		input   string
		want    LogLevel
		wantErr bool
	}{
		{"debug", `debug`, LevelDebug, false},
		{"info", `INFO`, LevelInfo, false},
		{"warn", `WARN`, LevelWarn, false},
		{"error", `ERROR`, LevelError, false},
		{"mixed case", `Warn`, LevelWarn, false},
		{"surrounding whitespace", `  info `, LevelInfo, false},
		{"unknown level", `verbose`, LevelUnknown, true},
		{"empty", ``, LevelUnknown, true},
		{"literal unknown is not a valid input", `unknown`, LevelUnknown, true},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			result, err := ParseLogLevel(d.input)
			if (err != nil) != d.wantErr {
				t.Fatalf("ParseLogLevel(%q) error = %v, wantErr %v", d.input, err, d.wantErr)
			}
			if result != d.want {
				t.Errorf("ParseLogLevel(%q) = %q, want %q", d.input, result, d.want)
			}
		})
	}
}

func TestLogEntry_ContainerType(t *testing.T) {
	data := []struct {
		name      string
		container string
		want      string
	}{
		{"web container", `web-1`, `web`},
		{"multi-digit index", `worker-12`, `worker`},
		{"router has no index", `router`, `router`},
		{"dash inside the type", `one-off-1234`, `one-off`},
		{"empty container", ``, ``},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			l := LogEntry{Container: d.container}

			r := l.ContainerType()
			if r != d.want {
				t.Errorf("got %q, want %q", r, d.want)
			}
		})
	}
}
