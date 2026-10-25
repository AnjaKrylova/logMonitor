package ingest

import (
	"testing"
	"time"

	"logmonitor/internal/logs"
)

func TestFakeLogSource_Stream(t *testing.T) {
	ctx := t.Context()
	fls := FakeLogSource{
		NumLines: 5,
	}
	ch := fls.Stream(ctx)

	var in []string
	for e := range ch {
		in = append(in, e)
	}

	if len(in) != 5 {
		t.Errorf("got %d lines, want 5", len(in))
	}
	for _, line := range in {
		entry, err := logs.ParseLog(line, time.Now())
		if err != nil {
			t.Errorf("ParseLog(%q) failed: %v", line, err)
		}
		if entry.Level == logs.LevelUnknown {
			t.Errorf("ParseLog(%q) returned unknown level", line)
		}
	}
}
