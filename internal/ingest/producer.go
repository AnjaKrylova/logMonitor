package ingest

import "logmonitor/internal/logs"

func Producer(n int) <-chan logs.LogEntry {
	ch := make(chan logs.LogEntry)
	go func() {
		for i := 0; i < n; i++ {
			ch <- logs.LogEntry{}
		}
		close(ch)
	}()
	return ch
}
