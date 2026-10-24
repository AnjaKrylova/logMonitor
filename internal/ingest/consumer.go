package ingest

import (
	"log/slog"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

func Consumer(ch <-chan logs.LogEntry, r storage.LogRepository) int {
	count := 0
	for e := range ch {
		err := r.Save(e)
		if err != nil {
			slog.Error("Save failed", "error", err)
			continue
		}
		count++
	}
	return count
}
