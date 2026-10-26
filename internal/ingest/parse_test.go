package ingest

import (
	"testing"

	"logmonitor/internal/logs"
)

func TestParse(t *testing.T) {
	ctx := t.Context()
	fls := FakeLogSource{
		NumLines: 5,
	}
	chIn := fls.Stream(ctx)
	chOut := make(chan logs.LogEntry, 5)

	Parse(chIn, chOut)

	close(chOut)
	var out []logs.LogEntry
	for e := range chOut {
		out = append(out, e)
	}
	if len(out) != 5 {
		t.Errorf("got %d entries, want 5", len(out))
	}
	for _, entry := range out {
		if len(entry.Raw) == 0 {
			t.Errorf("Parse returned empty Raw: %q", entry.Raw)
		}
		if entry.Level == logs.LevelUnknown {
			t.Errorf("Parse returned unknown level for entry: %+v", entry)
		}
	}
}
