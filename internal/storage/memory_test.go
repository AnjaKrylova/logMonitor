package storage_test

import (
	"reflect"
	"testing"
	"time"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

// 2025-09-08 13:12:58.867176204 +0200 CEST as an instant.
var timePrefixParsed = time.Date(2025, 9, 8, 11, 12, 58, 867176204, time.UTC)

func TestMemoryLogRepository_Find(t *testing.T) {
	data := []struct {
		name   string
		input  []logs.LogEntry
		filter storage.LogFilter
		want   []logs.LogEntry
	}{
		{"empty filter returns saved entry",
			[]logs.LogEntry{
				{
					Timestamp: timePrefixParsed,
					Level:     logs.LevelInfo,
					Message:   "npm WARN deprecated foo@1.0.0",
				},
			},
			storage.LogFilter{},
			[]logs.LogEntry{
				{
					Timestamp: timePrefixParsed,
					Level:     logs.LevelInfo,
					Message:   "npm WARN deprecated foo@1.0.0",
				},
			},
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
