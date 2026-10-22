package ingest

import (
	"context"
	"testing"

	"logmonitor/internal/logs"
)

func TestProducer(t *testing.T) {
	ctx := t.Context()
	numberOfEntries := 5

	ch := Producer(ctx, numberOfEntries)
	var in []logs.LogEntry
	for e := range ch {
		in = append(in, e)
	}

	if len(in) != numberOfEntries {
		t.Errorf("got %d entries, want %d", len(in), numberOfEntries)
	}
}

func TestProducer_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	numberOfEntries := 1_000_000

	ch := Producer(ctx, numberOfEntries)
	var in []logs.LogEntry
	for e := range ch {
		in = append(in, e)
		if len(in) == 5 {
			cancel()
		}
	}

	maxExpectedNumberOfEntries := 100
	if len(in) >= maxExpectedNumberOfEntries {
		t.Errorf("got %d entries after cancel, want fewer than %d", len(in), maxExpectedNumberOfEntries)
	}
}
