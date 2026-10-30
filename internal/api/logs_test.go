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

// 2025-09-08 13:12:58.867176204 +0200 CEST as an instant.
var timePrefixParsed = time.Date(2025, 9, 8, 11, 12, 58, 867176204, time.UTC)

func TestLogsHandler_ServeHTTP(t *testing.T) {
	routerLogEntry := logs.LogEntry{
		Timestamp: timePrefixParsed,
		Container: "router",
		Level:     logs.LevelInfo,
		Fields:    map[string]string{"status": "200"},
	}
	data := []struct {
		name      string
		wantCode  int
		wantEntry LogsResponse
	}{
		{"valid line", 200, LogsResponse{Logs: []logs.LogEntry{routerLogEntry}}},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			repo.Save(routerLogEntry)
			h := &LogsHandler{repo}
			req := httptest.NewRequest(http.MethodGet, "/logs", nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			code := w.Code
			if code != d.wantCode {
				t.Errorf("code got %d\nwant %d\n", code, d.wantCode)
			}
			var got LogsResponse
			err := json.Unmarshal(w.Body.Bytes(), &got)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if !reflect.DeepEqual(got, d.wantEntry) {
				t.Errorf("got %v, want %v", got.Logs, d.wantEntry)
			}
		})
	}
}
