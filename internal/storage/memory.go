package storage

import (
	"slices"

	"logmonitor/internal/logs"
)

type MemoryLogRepository struct {
	entries []logs.LogEntry
}

func (m *MemoryLogRepository) Save(e logs.LogEntry) error {
	m.entries = append(m.entries, e)
	return nil
}

func (m *MemoryLogRepository) Find(filter LogFilter) ([]logs.LogEntry, error) {
	return slices.Clone(m.entries), nil
}
