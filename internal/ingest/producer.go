package ingest

import (
	"context"

	"logmonitor/internal/logs"
)

func Producer(ctx context.Context, n int) <-chan logs.LogEntry {
	ch := make(chan logs.LogEntry)
	go func() {
		defer close(ch)
		for i := 0; i < n; i++ {
			select {
			case <-ctx.Done():
				return
			case ch <- logs.LogEntry{}:
			}
		}
	}()
	return ch
}
