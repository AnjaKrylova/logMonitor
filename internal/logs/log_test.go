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
