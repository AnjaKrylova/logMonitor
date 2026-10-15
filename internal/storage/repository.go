package storage

import (
	"time"

	"logmonitor/internal/logs"
)

type LogFilter struct {
	From time.Time // inclusive
	To   time.Time // exclusive
	App  string
}

type LogRepository interface {
	Save(entry logs.LogEntry) error
	Find(filter LogFilter) ([]logs.LogEntry, error)
}
