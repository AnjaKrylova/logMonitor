package ingest

import (
	"log/slog"
	"time"

	"logmonitor/internal/logs"
)

func Parse(in <-chan string, out chan<- logs.LogEntry) {
	for line := range in {
		entry, err := logs.ParseLog(line, time.Now())
		if err != nil {
			slog.Warn("failed to parse log line", "error", err)
		}
		out <- entry
	}
}
