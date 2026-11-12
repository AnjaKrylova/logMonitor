package storage

import (
	"time"

	"logmonitor/internal/logs"
)

type LogFilter struct {
	From          time.Time // inclusive
	To            time.Time // exclusive
	App           string
	ContainerType string
	Limit         int // max entries, newest first; 0 = no limit
	After         Position
}

type Position struct {
	Timestamp time.Time
	ID        string
}

func (f LogFilter) matches(e logs.LogEntry) bool {
	if len(f.App) > 0 && e.App != f.App {
		return false
	}
	if len(f.ContainerType) > 0 && e.ContainerType() != f.ContainerType {
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
	if !f.After.Timestamp.IsZero() && !isAfter(e, f.After) {
		return false
	}

	return true
}

type LogRepository interface {
	// Save stores e and assigns it a unique ID.
	Save(e logs.LogEntry) error
	// Find returns the entries matching f, newest first;
	// entries with the same timestamp are ordered by ID, descending.
	Find(f LogFilter) ([]logs.LogEntry, error)
}

func isAfter(e logs.LogEntry, p Position) bool {
	return e.Timestamp.Before(p.Timestamp) || (e.Timestamp.Equal(p.Timestamp) && e.ID < p.ID)
}
