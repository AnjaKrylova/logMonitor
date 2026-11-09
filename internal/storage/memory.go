package storage

import (
	"slices"
	"sync"

	"logmonitor/internal/logs"
)

type MemoryLogRepository struct {
	mu      sync.Mutex
	entries []logs.LogEntry
}

func (m *MemoryLogRepository) Save(e logs.LogEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, e)
	return nil
}

func (m *MemoryLogRepository) Find(f LogFilter) ([]logs.LogEntry, error) {
	var result []logs.LogEntry
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if f.matches(e) {
			result = append(result, e)
		}
	}
	slices.SortStableFunc(result, func(a, b logs.LogEntry) int {
		r := a.Timestamp.Compare(b.Timestamp)
		return -r
	})
	if f.Limit != 0 && f.Limit < len(result) {
		return result[:f.Limit], nil
	}
	return result, nil
}
