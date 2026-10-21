package storage_test

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

// 2025-09-08 13:12:58.867176204 +0200 CEST as an instant.
var timePrefixParsed = time.Date(2025, 9, 8, 11, 12, 58, 867176204, time.UTC)

func TestMemoryLogRepository_Find(t *testing.T) {
	// Fixtures: an hour before, at, and an hour after timePrefixParsed.
	before := logs.LogEntry{Timestamp: timePrefixParsed.Add(-time.Hour), Message: "before"}
	at := logs.LogEntry{Timestamp: timePrefixParsed, Message: "at"}
	after := logs.LogEntry{Timestamp: timePrefixParsed.Add(time.Hour), Message: "after"}
	app1 := logs.LogEntry{App: "app1", Level: logs.LevelInfo}
	app2 := logs.LogEntry{App: "app2", Level: logs.LevelInfo}

	// Boundary fixtures: neighbours 1 ns and 2 ns around timePrefixParsed.
	atMinus2 := logs.LogEntry{Timestamp: timePrefixParsed.Add(-2 * time.Nanosecond), Message: "at-2"}
	atMinus1 := logs.LogEntry{Timestamp: timePrefixParsed.Add(-time.Nanosecond), Message: "at-1"}
	atPlus1 := logs.LogEntry{Timestamp: timePrefixParsed.Add(time.Nanosecond), Message: "at+1"}
	atPlus2 := logs.LogEntry{Timestamp: timePrefixParsed.Add(2 * time.Nanosecond), Message: "at+2"}

	// appAt builds an entry for app at timePrefixParsed shifted by offset.
	appAt := func(app string, offset time.Duration) logs.LogEntry {
		return logs.LogEntry{Timestamp: timePrefixParsed.Add(offset), App: app}
	}

	data := []struct {
		name   string
		input  []logs.LogEntry
		filter storage.LogFilter
		want   []logs.LogEntry
	}{
		{
			"empty filter returns saved entry",
			[]logs.LogEntry{at},
			storage.LogFilter{},
			[]logs.LogEntry{at},
		},
		{
			"filter with no matching entries returns nil",
			[]logs.LogEntry{app1},
			storage.LogFilter{App: "nonexistent-app"},
			nil,
		},
		{
			"empty repository returns nil",
			nil,
			storage.LogFilter{},
			nil,
		},
		{
			"From and To return only entries in the range",
			[]logs.LogEntry{before, at, after},
			storage.LogFilter{From: timePrefixParsed.Add(-30 * time.Minute), To: timePrefixParsed.Add(30 * time.Minute)},
			[]logs.LogEntry{at},
		},
		{
			"From returns only entries at or after From",
			[]logs.LogEntry{before, at, after},
			storage.LogFilter{From: timePrefixParsed.Add(-30 * time.Minute)},
			[]logs.LogEntry{at, after},
		},
		{
			"To returns only entries before To",
			[]logs.LogEntry{before, at, after},
			storage.LogFilter{To: timePrefixParsed.Add(30 * time.Minute)},
			[]logs.LogEntry{before, at},
		},
		{
			"App returns only entries with the matching App",
			[]logs.LogEntry{app1, app2},
			storage.LogFilter{App: "app1"},
			[]logs.LogEntry{app1},
		},
		{
			"From is inclusive, To is exclusive",
			[]logs.LogEntry{atMinus2, atMinus1, at, atPlus1, atPlus2},
			storage.LogFilter{From: atMinus1.Timestamp, To: atPlus1.Timestamp},
			[]logs.LogEntry{atMinus1, at},
		},
		{
			"App and time range must both match",
			[]logs.LogEntry{
				appAt("app1", -time.Hour), appAt("app2", -time.Hour),
				appAt("app1", 0), appAt("app2", 0),
				appAt("app1", time.Hour), appAt("app2", time.Hour),
			},
			storage.LogFilter{From: timePrefixParsed.Add(-30 * time.Minute), To: timePrefixParsed.Add(30 * time.Minute), App: "app2"},
			[]logs.LogEntry{appAt("app2", 0)},
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			for _, entry := range d.input {
				err := repo.Save(entry)
				if err != nil {
					t.Fatalf("Save(%+v) failed: %v", entry, err)
				}
			}

			result, err := repo.Find(d.filter)
			if err != nil {
				t.Fatalf("Find(%+v) failed: %v", d.filter, err)
			}

			if !reflect.DeepEqual(result, d.want) {
				t.Errorf("got %+v, want %+v", result, d.want)
			}
		})
	}
}

func TestMemoryLogRepository_FindReturnsCopy(t *testing.T) {
	var repo storage.LogRepository = &storage.MemoryLogRepository{}
	entry := logs.LogEntry{
		Timestamp: timePrefixParsed,
		Level:     logs.LevelInfo,
		Message:   "npm WARN deprecated foo@1.0.0",
	}
	err := repo.Save(entry)
	if err != nil {
		t.Fatalf("Save(%+v) failed: %v", entry, err)
	}
	filter := storage.LogFilter{}
	result, err := repo.Find(filter)
	if err != nil {
		t.Fatalf("Find(%+v) failed: %v", filter, err)
	}
	if len(result) != 1 {
		t.Fatalf("Find returned %d entries, want 1", len(result))
	}
	result[0].Message = "new value"
	result2, err := repo.Find(filter)
	if err != nil {
		t.Fatalf("Find(%+v) failed: %v", filter, err)
	}
	if len(result2) != 1 {
		t.Fatalf("Find returned %d entries, want 1", len(result2))
	}
	if result2[0].Message != entry.Message {
		t.Errorf("stored entry was modified through Find result: got %q, want %q", result2[0].Message, entry.Message)
	}
}

func TestMemoryLogRepository_ConcurrentSave(t *testing.T) {
	var repo storage.LogRepository = &storage.MemoryLogRepository{}

	var wg sync.WaitGroup
	for i := 0; i < 10_000; i++ {
		wg.Go(func() {
			repo.Save(logs.LogEntry{})
		})
	}
	wg.Wait()

	filter := storage.LogFilter{}
	result, err := repo.Find(filter)
	if err != nil {
		t.Fatalf("Find(%+v) failed: %v", filter, err)
	}
	if len(result) != 10_000 {
		t.Fatalf("Find returned %d entries, want 10_000", len(result))
	}
}
