package storage

import "logmonitor/internal/logs"

type MemoryLogRepository struct {
	logEntry *logs.LogEntry
}

func (p MemoryLogRepository) Save(entry logs.LogEntry) error {
	return nil
}

func (p MemoryLogRepository) Find(filter LogFilter) ([]logs.LogEntry, error) {
	return []logs.LogEntry{}, nil
}
