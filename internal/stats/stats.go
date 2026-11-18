package stats

import "logmonitor/internal/logs"

// GroupKey identifies one dashboard card: app + container type + level + message template.
type GroupKey struct {
	App           string
	ContainerType string
	Level         logs.LogLevel
	Message       string // the template, e.g. "User %s not found"
}

// CountByGroup counts entries per GroupKey.
func CountByGroup(entries []logs.LogEntry) map[GroupKey]int {
	counts := map[GroupKey]int{}
	for _, e := range entries {
		counts[GroupKey{
			App:           e.App,
			ContainerType: e.ContainerType(),
			Level:         e.Level,
			Message:       e.Message,
		}]++
	}
	return counts
}
