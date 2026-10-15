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
			for _, v := range d.input {
				err := repo.Save(v)
				if err != nil {
					t.Fatalf("Save(%+v) failed: %v", d.input, err)
				}
			}

			result, err := repo.Find(d.filter)
			if err != nil {
				t.Fatalf("Find(%+v) failed: %v", d.filter, err)
			}

			if !reflect.DeepEqual(result, d.want) {
				t.Errorf("got %v, want %v", result, d.want)
			}
		})
	}
}
