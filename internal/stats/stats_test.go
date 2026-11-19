package stats

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"logmonitor/internal/logs"
)

// 2025-09-08 10:15:00 UTC
var logTime = time.Date(2025, 9, 8, 10, 15, 0, 0, time.UTC)

func TestCountByGroup(t *testing.T) {
	data := []struct {
		name    string
		entries []logs.LogEntry
		want    map[GroupKey]int
	}{
		{"no entries",
			nil,
			nil,
		},
		{"3 entries in the same group",
			[]logs.LogEntry{
				{Container: "web-1", Level: logs.LevelError, Message: "boom"},
				{Container: "web-1", Level: logs.LevelError, Message: "boom"},
				{Container: "web-1", Level: logs.LevelError, Message: "boom"},
			},
			map[GroupKey]int{
				{ContainerType: "web", Level: logs.LevelError, Message: "boom"}: 3,
			},
		},
		{"entries that differ in just one field",
			[]logs.LogEntry{
				{Container: "web-1", Level: logs.LevelWarn, Message: "boom"},
				{Container: "web-2", Level: logs.LevelError, Message: "boom"},
				{Container: "web-1", Level: logs.LevelError, Message: "test"},
				{Container: "web-1", Level: logs.LevelError, Message: "boom"},
			},
			map[GroupKey]int{
				{ContainerType: "web", Level: logs.LevelWarn, Message: "boom"}:  1,
				{ContainerType: "web", Level: logs.LevelError, Message: "boom"}: 2,
				{ContainerType: "web", Level: logs.LevelError, Message: "test"}: 1,
			},
		},
		{"different apps are different groups",
			[]logs.LogEntry{
				{App: "app1", Container: "worker-1", Level: logs.LevelError, Message: "boom"},
				{App: "app2", Container: "worker-1", Level: logs.LevelError, Message: "boom"},
				{App: "app2", Container: "worker-2", Level: logs.LevelError, Message: "boom"},
			},
			map[GroupKey]int{
				{App: "app1", ContainerType: "worker", Level: logs.LevelError, Message: "boom"}: 1,
				{App: "app2", ContainerType: "worker", Level: logs.LevelError, Message: "boom"}: 2,
			},
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			got := CountByGroup(d.entries)

			// "No groups" may come back as nil or as an empty map; both mean the same.
			if diff := cmp.Diff(d.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("CountByGroup mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHourlyByGroup(t *testing.T) {
	data := []struct {
		name    string
		entries []logs.LogEntry
		want    map[GroupKey][24]int
	}{
		{"3 entries in the same group",
			[]logs.LogEntry{
				{Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: logTime},
				{Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: logTime.Add(15 * time.Minute)},
				{Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: logTime.Add(4 * time.Hour)},
			},
			map[GroupKey][24]int{
				{ContainerType: "web", Level: logs.LevelError, Message: "boom"}: {10: 2, 14: 1},
			},
		},
		{"no entries", nil, nil},
		{"two groups each get their own hours",
			[]logs.LogEntry{
				{App: "app1", Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: logTime},
				{App: "app2", Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: logTime},
				{App: "app2", Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: logTime.Add(time.Hour)},
			},
			map[GroupKey][24]int{
				{App: "app1", ContainerType: "web", Level: logs.LevelError, Message: "boom"}: {10: 1},
				{App: "app2", ContainerType: "web", Level: logs.LevelError, Message: "boom"}: {10: 1, 11: 1},
			},
		},
		{"first and last hour of the day",
			[]logs.LogEntry{
				{Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: time.Date(2025, 9, 8, 0, 0, 0, 0, time.UTC)},
				{Container: "web-1", Level: logs.LevelError, Message: "boom", Timestamp: time.Date(2025, 9, 8, 23, 59, 59, 0, time.UTC)},
			},
			map[GroupKey][24]int{
				{ContainerType: "web", Level: logs.LevelError, Message: "boom"}: {0: 1, 23: 1},
			},
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			got := HourlyByGroup(d.entries)

			// "No groups" may come back as nil or as an empty map; both mean the same.
			if diff := cmp.Diff(d.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("HourlyByGroup mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDashboard(t *testing.T) {
	data := []struct {
		name    string
		entries []logs.LogEntry
		want    []Group
	}{{"different counts",
		[]logs.LogEntry{
			{App: "app1", Container: "web-1", Level: logs.LevelError, Message: "boom"},
			{App: "app2", Container: "web-1", Level: logs.LevelError, Message: "boom"},
			{App: "app2", Container: "web-1", Level: logs.LevelError, Message: "boom"},
		},
		[]Group{
			{Key: GroupKey{App: "app2", ContainerType: "web", Level: logs.LevelError, Message: "boom"}, Count: 2, Hours: [24]int{0: 2}},
			{Key: GroupKey{App: "app1", ContainerType: "web", Level: logs.LevelError, Message: "boom"}, Count: 1, Hours: [24]int{0: 1}},
		},
	}, {"same counts, different apps",
		[]logs.LogEntry{
			{App: "appB", Container: "web-1", Level: logs.LevelError, Message: "boom"},
			{App: "appA", Container: "web-1", Level: logs.LevelError, Message: "boom"},
		},
		[]Group{
			{Key: GroupKey{App: "appA", ContainerType: "web", Level: logs.LevelError, Message: "boom"}, Count: 1, Hours: [24]int{0: 1}},
			{Key: GroupKey{App: "appB", ContainerType: "web", Level: logs.LevelError, Message: "boom"}, Count: 1, Hours: [24]int{0: 1}},
		},
	}}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			got := Dashboard(d.entries)

			if diff := cmp.Diff(d.want, got); diff != "" {
				t.Errorf("Dashboard mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
