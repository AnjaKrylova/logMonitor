package ingest

import (
	"testing"

	"logmonitor/internal/storage"
)

func TestConsumer(t *testing.T) {
	ch := Producer(5)
	var repo storage.LogRepository = &storage.MemoryLogRepository{}
	count := Consumer(ch, repo)
	f := storage.LogFilter{}
	result, err := repo.Find(f)
	if err != nil {
		t.Fatalf("Find(%+v) failed: %v", f, err)
	}
	if len(result) != 5 {
		t.Errorf("TestConsumer number of saved entries\n got: %v\nwant: %v", len(result), 5)
	}
	if count != 5 {
		t.Errorf("TestConsumer returned count\n got: %v\nwant: %v", count, 5)
	}
}
