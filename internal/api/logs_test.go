package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

// 2025-09-08 13:45 CEST (11:45 UTC): the time of the newest test entry.
var logTime = time.Date(2025, 9, 8, 11, 45, 0, 0, time.UTC)

func TestLogsHandler_ServeHTTP(t *testing.T) {
	data := []struct {
		name         string
		requestURL   string
		savedEntries []logs.LogEntry
		wantCode     int
		wantResponse LogsResponse
	}{
		{
			"valid line",
			"/logs",
			[]logs.LogEntry{
				{
					Timestamp: logTime,
					Container: "router",
					Level:     logs.LevelInfo,
					Fields:    map[string]string{"status": "200"},
				},
			},
			200,
			LogsResponse{
				Logs: []logs.LogEntry{
					{
						Timestamp: logTime,
						Container: "router",
						Level:     logs.LevelInfo,
						Fields:    map[string]string{"status": "200"},
					},
				},
			},
		},
		{
			"valid line with app filter",
			"/logs?app=demo-app",
			[]logs.LogEntry{
				{
					Timestamp: logTime,
					Container: "web-1",
					App:       "test1-app",
				},
				{
					Timestamp: logTime,
					Container: "web-1",
					App:       "demo-app",
				},
			},
			200,
			LogsResponse{
				Logs: []logs.LogEntry{
					{
						Timestamp: logTime,
						Container: "web-1",
						App:       "demo-app",
					},
				},
			},
		},
		{"valid line with from filter",
			"/logs?from=2025-09-08T11:35:00Z",
			[]logs.LogEntry{
				{
					Timestamp: logTime,
					Container: "web-1",
					App:       "test1-app",
				},
				{
					Timestamp: logTime.Add(-30 * time.Minute),
					Container: "web-1",
					App:       "demo-app",
				},
			},
			200,
			LogsResponse{
				Logs: []logs.LogEntry{
					{
						Timestamp: logTime,
						Container: "web-1",
						App:       "test1-app",
					},
				},
			},
		},
		{"valid line with to filter",
			"/logs?to=2025-09-08T11:35:00Z",
			[]logs.LogEntry{
				{
					Timestamp: logTime,
					Container: "web-1",
					App:       "test1-app",
				},
				{
					Timestamp: logTime.Add(-30 * time.Minute),
					Container: "web-1",
					App:       "demo-app",
				},
			},
			200,
			LogsResponse{
				Logs: []logs.LogEntry{
					{
						Timestamp: logTime.Add(-30 * time.Minute),
						Container: "web-1",
						App:       "demo-app",
					},
				},
			},
		},
		{
			"invalid from parameter",
			"/logs?from=yesterday",
			[]logs.LogEntry{},
			400,
			LogsResponse{},
		},
		{
			"invalid to parameter",
			"/logs?to=tomorrow",
			[]logs.LogEntry{},
			400,
			LogsResponse{},
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			for _, e := range d.savedEntries {
				repo.Save(e)
			}
			h := &LogsHandler{repo}
			req := httptest.NewRequest(http.MethodGet, d.requestURL, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			code := w.Code
			if code != d.wantCode {
				t.Errorf("code got %d\nwant %d\n", code, d.wantCode)
			}
			var got LogsResponse
			if d.wantCode != http.StatusOK {
				return
			}
			err := json.Unmarshal(w.Body.Bytes(), &got)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !reflect.DeepEqual(got, d.wantResponse) {
				t.Errorf("got %v\nwant %v", got.Logs, d.wantResponse.Logs)
			}
		})
	}
}
