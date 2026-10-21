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
			ch := Producer(d.numberOfEntries)
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			count := Consumer(ch, repo)
			f := storage.LogFilter{}
			result, err := repo.Find(f)
			if err != nil {
				t.Fatalf("Find(%+v) failed: %v", f, err)
			}
			if len(result) != d.numberOfEntries {
				t.Errorf("TestConsumer number of saved entries\n got: %v\nwant: %v", len(result), d.numberOfEntries)
			}
			if count != d.numberOfEntries {
				t.Errorf("TestConsumer returned count\n got: %v\nwant: %v", count, d.numberOfEntries)
			}
		})
	}

}
