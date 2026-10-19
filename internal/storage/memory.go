package storage

import (
	"logmonitor/internal/logs"
)

type MemoryLogRepository struct {
	entries []logs.LogEntry
}

func (m *MemoryLogRepository) Save(e logs.LogEntry) error {
	m.entries = append(m.entries, e)
	return nil
}

func (m *MemoryLogRepository) Find(f LogFilter) ([]logs.LogEntry, error) {
	var result []logs.LogEntry
	for _, e := range m.entries {
		if f.matches(e) {
			result = append(result, e)
		}
	}
	return result, nil
}
