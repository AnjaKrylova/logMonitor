package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"logmonitor/internal/logs"
)

var routerLine = `2026-06-24 15:02:19.446989072 +0200 CEST [router] method=GET path="/password" status=200 duration=0.030s`

func TestHandler_ServeHTTP(t *testing.T) {
	validBody, err := json.Marshal(Request{Line: routerLine})
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	data := []struct {
		name      string
		body      string // raw request body, sent as-is
		wantCode  int
		wantRaw   string // expected raw log line in the channel
		wantEntry bool   // should an entry arrive on the channel
	}{

		{"valid line", string(validBody), 202, routerLine, true},
		{"line without timestamp", `{"line": "just some text"}`, 202, `just some text`, true},
		{"invalid JSON", `not json`, 400, "", false},
		{"line that isn't a string", `{"line": 42}`, 400, "", false},
		{"line missing", `{}`, 400, "", false},
		{"line empty", `{"line": ""}`, 400, "", false},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			ch := make(chan logs.LogEntry, 1)
			h := &Handler{ch}
			body := strings.NewReader(d.body)
			req := httptest.NewRequest(http.MethodPost, "/ingest", body)
			w := httptest.NewRecorder()

			h.ServeHTTP(w, req)

			code := w.Code
			if code != d.wantCode {
				t.Errorf("code got %d\nwant %d\n", code, d.wantCode)
			}
			var in []logs.LogEntry
			select {
			case e := <-ch:
				in = append(in, e)
			default:
			}
			if d.wantEntry {
				if len(in) != 1 {
					t.Fatalf("got %d entries, want %d", len(in), 1)
				}
				if in[0].Raw != d.wantRaw {
					t.Errorf("got %q, want %q", in[0].Raw, d.wantRaw)
				}
			} else {
				if len(in) != 0 {
					t.Fatalf("got %d entries, want %d", len(in), 0)
				}
			}
		})
	}
}
