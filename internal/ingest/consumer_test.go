package ingest

import (
	"testing"

	"logmonitor/internal/storage"
)

func TestConsumer(t *testing.T) {
	data := []struct {
		name            string
		numberOfEntries int
	}{
		{"0 entries", 0},
		{"5 entries", 5},
		{"1_000 entries", 1_000},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			ctx := t.Context()
			ch := Producer(ctx, d.numberOfEntries)
			var repo storage.LogRepository = &storage.MemoryLogRepository{}

			count := Consumer(ch, repo)

			f := storage.LogFilter{}
			result, err := repo.Find(f)
			if err != nil {
				t.Fatalf("Find(%+v) failed: %v", f, err)
			}
			if len(result) != d.numberOfEntries {
				t.Errorf("repository has %d entries, want %d", len(result), d.numberOfEntries)
			}
			if count != d.numberOfEntries {
				t.Errorf("Consumer returned %d, want %d", count, d.numberOfEntries)
			}
		})
	}
}
