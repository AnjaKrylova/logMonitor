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
	Save(e logs.LogEntry) error
	Find(f LogFilter) ([]logs.LogEntry, error)
}
