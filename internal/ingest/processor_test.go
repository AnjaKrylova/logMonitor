package ingest

import (
	"context"
	"testing"
	"time"

	"logmonitor/internal/storage"
)

func TestProcessor_Run(t *testing.T) {
	data := []struct {
		name            string
		numberOfWorkers int
	}{
		{"1 worker", 1},
		{"2 worker", 2},
		{"4 worker", 4},
		{"8 worker", 8},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			numberOfEntries := 1_000
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			p := Processor{
				NumberOfWorkers: d.numberOfWorkers,
				Repository:      repo,
			}
			ctx := t.Context()
			ch := Producer(ctx, numberOfEntries)

			p.Run(ch)

			f := storage.LogFilter{}
			result, err := repo.Find(f)
			if err != nil {
				t.Fatalf("Find(%+v) failed: %v", f, err)
			}
			if len(result) != numberOfEntries {
				t.Errorf("repository has %d entries, want %d", len(result), numberOfEntries)
			}
		})
	}
}

func BenchmarkProcessor_Run(b *testing.B) {
	data := []struct {
		name            string
		numberOfWorkers int
	}{
		{"1 worker", 1},
		{"2 worker", 2},
		{"4 worker", 4},
		{"8 worker", 8},
	}

	for _, d := range data {
		b.Run(d.name, func(b *testing.B) {
			numberOfEntries := 100_000
			var repo storage.LogRepository

			for b.Loop() {
				repo = &storage.MemoryLogRepository{}
				p := Processor{
					NumberOfWorkers: d.numberOfWorkers,
					Repository:      repo,
				}
				ctx := b.Context()
				ch := Producer(ctx, numberOfEntries)

				p.Run(ch)
			}

			f := storage.LogFilter{}
			result, err := repo.Find(f)
			if err != nil {
				b.Fatalf("Find(%+v) failed: %v", f, err)
			}
			if len(result) != numberOfEntries {
				b.Errorf("repository has %d entries, want %d", len(result), numberOfEntries)
			}
		})
	}
}

func TestProcessor_Run_Timeout(t *testing.T) {
	numberOfEntries := 10_000_000
	numberOfWorkers := 1
	var repo storage.LogRepository = &storage.MemoryLogRepository{}
	p := Processor{
		NumberOfWorkers: numberOfWorkers,
		Repository:      repo,
	}
	shortDuration := 50 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), shortDuration)
	defer cancel()
	ch := Producer(ctx, numberOfEntries)
	start := time.Now()

	p.Run(ch)
	elapsed := time.Since(start)

	oneSec := 1 * time.Second
	if elapsed > oneSec {
		t.Fatalf("Run() took too long: %v, want less than %v", elapsed, oneSec)
	}
	f := storage.LogFilter{}
	result, err := repo.Find(f)
	if err != nil {
		t.Fatalf("Find(%+v) failed: %v", f, err)
	}
	maxExpectedNumberOfEntries := 100_000
	if len(result) >= maxExpectedNumberOfEntries {
		t.Errorf("got %d entries after cancel, want fewer than %d", len(result), maxExpectedNumberOfEntries)
	}
}
