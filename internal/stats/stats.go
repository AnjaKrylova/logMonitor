package stats

import (
	"cmp"
	"slices"

	"logmonitor/internal/logs"
)

// GroupKey identifies one dashboard card: app + container type + level + message template.
type GroupKey struct {
	App           string
	ContainerType string
	Level         logs.LogLevel
	Message       string // the template, e.g. "User %s not found"
}

// Group is one dashboard card.
type Group struct {
	Key   GroupKey
	Count int
	Hours [24]int
}

// CountByGroup counts entries per GroupKey.
func CountByGroup(entries []logs.LogEntry) map[GroupKey]int {
	counts := map[GroupKey]int{}
	for _, e := range entries {
		key := keyOf(e)
		counts[key]++
	}

	return counts
}

// HourlyByGroup counts entries per GroupKey and UTC hour (index 0–23).
func HourlyByGroup(entries []logs.LogEntry) map[GroupKey][24]int {
	hours := map[GroupKey][24]int{}
	for _, e := range entries {
		key := keyOf(e)
		h := e.Timestamp.Hour()
		v := hours[key]
		v[h]++
		hours[key] = v
	}

	return hours
}

func keyOf(e logs.LogEntry) GroupKey {
	return GroupKey{
		App:           e.App,
		ContainerType: e.ContainerType(),
		Level:         e.Level,
		Message:       e.Message,
	}
}

// Dashboard returns one Group per GroupKey, biggest Count first;
// equal counts are ordered by App, then Message.
func Dashboard(entries []logs.LogEntry) []Group {
	counts := CountByGroup(entries)
	hours := HourlyByGroup(entries)
	var groups []Group
	for key, count := range counts {
		groups = append(groups, Group{Key: key, Count: count, Hours: hours[key]})
	}
	slices.SortFunc(groups, func(a, b Group) int {
		return cmp.Or(
			cmp.Compare(b.Count, a.Count), // biggest first
			cmp.Compare(a.Key.App, b.Key.App),
			cmp.Compare(a.Key.Message, b.Key.Message),
		)
	})
	return groups
}
