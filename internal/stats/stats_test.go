package stats

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"logmonitor/internal/logs"
)

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
