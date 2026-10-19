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

func (f LogFilter) matches(e logs.LogEntry) bool {
	if len(f.App) > 0 && e.App != f.App {
		return false
	}
	beforeFrom := e.Timestamp.Before(f.From)
	if !f.From.IsZero() && beforeFrom {
		return false
	}
	atOrAfterTo := !e.Timestamp.Before(f.To)
	if !f.To.IsZero() && atOrAfterTo {
		return false
	}
	return true
}

type LogRepository interface {
	Save(e logs.LogEntry) error
	Find(f LogFilter) ([]logs.LogEntry, error)
}
