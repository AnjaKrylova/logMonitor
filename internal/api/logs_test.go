package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

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
		{"limit is not a number", "/logs?limit=abc", nil, 400, LogsResponse{}},
		{"limit is zero", "/logs?limit=0", nil, 400, LogsResponse{}},
		{"limit above the maximum", "/logs?limit=501", nil, 400, LogsResponse{}},
		{"cursor is not a cursor", "/logs?cursor=not-a-cursor!", nil, 400, LogsResponse{}},
		{"nonexistent app filter",
			"/logs?app=nonexistent-app",
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
				Logs: []logs.LogEntry{},
			},
		},
		{"container type filter",
			"/logs?container_type=worker",
			[]logs.LogEntry{
				{
					Timestamp: logTime,
					Container: "web-1",
					App:       "demo-app",
				},
				{
					Timestamp: logTime.Add(-30 * time.Minute),
					Container: "worker-1",
					App:       "demo-app",
				},
			},
			200,
			LogsResponse{
				Logs: []logs.LogEntry{
					{
						Timestamp: logTime.Add(-30 * time.Minute),
						Container: "worker-1",
						App:       "demo-app",
					},
				},
			},
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
			// IDs are random (assigned by Save), so compare everything except ID.
			if diff := cmp.Diff(d.wantResponse, got, cmpopts.IgnoreFields(logs.LogEntry{}, "ID")); diff != "" {
				t.Errorf("response mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLogsHandler_ServeHTTP_Limit(t *testing.T) {
	data := []struct {
		name       string
		requestURL string
		wantCount  int
	}{
		{"default limit is 50", "/logs", 50},
		{"explicit limit", "/logs?limit=2", 2},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			for i := range 60 {
				repo.Save(logs.LogEntry{Timestamp: logTime.Add(time.Duration(i) * time.Minute)})
			}
			h := &LogsHandler{repo}
			req := httptest.NewRequest(http.MethodGet, d.requestURL, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			var got LogsResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if len(got.Logs) != d.wantCount {
				t.Errorf("got %d entries, want %d", len(got.Logs), d.wantCount)
			}
		})
	}
}

func TestLogsHandler_ServeHTTP_NextCursor(t *testing.T) {
	// 3 entries saved; a full page (len == limit) may have more after it.
	data := []struct {
		name       string
		requestURL string
		wantCursor bool
	}{
		{"full page has a next cursor", "/logs?limit=2", true},
		{"last page has no next cursor", "/logs?limit=5", false},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			for i := range 3 {
				repo.Save(logs.LogEntry{Timestamp: logTime.Add(time.Duration(i) * time.Minute)})
			}
			h := &LogsHandler{repo}
			req := httptest.NewRequest(http.MethodGet, d.requestURL, nil)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			var got LogsResponse
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if gotCursor := got.NextCursor != ""; gotCursor != d.wantCursor {
				t.Errorf("nextCursor present = %v, want %v (nextCursor %q)", gotCursor, d.wantCursor, got.NextCursor)
			}
		})
	}
}

func TestLogsHandler_ServeHTTP_Paging(t *testing.T) {
	var repo storage.LogRepository = &storage.MemoryLogRepository{}
	for i := range 5 {
		repo.Save(logs.LogEntry{Timestamp: logTime.Add(time.Duration(i) * time.Minute)})
	}
	all, err := repo.Find(storage.LogFilter{})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	h := &LogsHandler{repo}

	// 5 entries with limit 2 → pages of 2, 2, 1. Following the cursors must
	// return every entry exactly once, in Find's order.
	page1 := getLogs(t, h, "/logs?limit=2")
	page2 := getLogs(t, h, "/logs?limit=2&cursor="+page1.NextCursor)
	page3 := getLogs(t, h, "/logs?limit=2&cursor="+page2.NextCursor)

	pages := []struct {
		name       string
		got        LogsResponse
		want       []logs.LogEntry
		wantCursor bool
	}{
		{"page 1", page1, all[0:2], true},
		{"page 2", page2, all[2:4], true},
		{"page 3 (last)", page3, all[4:], false},
	}
	for _, p := range pages {
		if diff := cmp.Diff(p.want, p.got.Logs); diff != "" {
			t.Errorf("%s mismatch (-want +got):\n%s", p.name, diff)
		}
		if gotCursor := p.got.NextCursor != ""; gotCursor != p.wantCursor {
			t.Errorf("%s: nextCursor present = %v, want %v", p.name, gotCursor, p.wantCursor)
		}
	}
}

// getLogs sends GET url to h and decodes the JSON response.
func getLogs(t *testing.T, h http.Handler, url string) LogsResponse {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))

	var resp LogsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GET %s: Unmarshal failed: %v", url, err)
	}
	return resp
}

func TestLogsHandler_ServeLog(t *testing.T) {
	var repo storage.LogRepository = &storage.MemoryLogRepository{}
	repo.Save(logs.LogEntry{Timestamp: logTime, Container: "web-1", Message: "User %s not found", MessageParams: []string{"123"}})
	all, err := repo.Find(storage.LogFilter{})
	if err != nil {
		t.Fatalf("Find failed: %v", err)
	}
	h := &LogsHandler{repo}

	data := []struct {
		name     string
		id       string
		wantCode int
	}{
		{"known ID returns the entry", all[0].ID, http.StatusOK},
		{"unknown ID returns 404", "no-such-id", http.StatusNotFound},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/logs/"+d.id, nil)
			// The router fills {id} only when it routes the request; calling
			// the handler directly, the test sets the path value itself.
			req.SetPathValue("id", d.id)
			w := httptest.NewRecorder()

			h.ServeLog(w, req)

			if w.Code != d.wantCode {
				t.Fatalf("code got %d, want %d", w.Code, d.wantCode)
			}
			if d.wantCode != http.StatusOK {
				return
			}
			var got logs.LogEntry
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if diff := cmp.Diff(all[0], got); diff != "" {
				t.Errorf("entry mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
