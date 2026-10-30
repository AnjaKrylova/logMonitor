package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"logmonitor/internal/logs"
)

var routerLine = `2026-06-24 15:02:19.446989072 +0200 CEST[router] method=GET path="/password" status=200 duration=0.030s`

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

// When the queue is full, ServeHTTP blocks until there is room, then
// delivers the entry and answers 202: nothing is dropped.
func TestHandler_ServeHTTP_QueueFull(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		validBody, err := json.Marshal(Request{Line: routerLine})
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}

		// Fill the queue so the handler's send has to wait.
		ch := make(chan logs.LogEntry, 1)
		filler := logs.LogEntry{Message: "filler"}
		ch <- filler

		h := &Handler{ch}
		body := strings.NewReader(string(validBody))
		req := httptest.NewRequest(http.MethodPost, "/ingest", body)
		w := httptest.NewRecorder()

		// The handler blocks on the full queue, so run it in the background;
		// done is closed when it returns.
		done := make(chan struct{})
		go func() {
			defer close(done)
			h.ServeHTTP(w, req)
		}()

		// Make room: removing the filler unblocks the handler.
		synctest.Wait()
		<-ch

		// Wait for the handler to finish, but fail instead of hanging.
		timeout := time.Second
		select {
		case <-done:
		case <-time.After(timeout):
			t.Fatalf("ServeHTTP did not return within %v", timeout)
		}

		if w.Code != http.StatusAccepted {
			t.Errorf("status = %d, want %d", w.Code, http.StatusAccepted)
		}
		delivered := <-ch
		if delivered.Raw != routerLine {
			t.Errorf("delivered Raw = %q, want %q", delivered.Raw, routerLine)
		}
	})
}
