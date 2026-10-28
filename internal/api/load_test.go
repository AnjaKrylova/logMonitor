package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"logmonitor/internal/ingest"
	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

// Simulates many Scalingo containers draining into one POST /ingest at the
// same time, through a real HTTP server and the real pipeline, and checks
// that every line is saved. Run with -v to see the duration per row.
func TestHandler_ServeHTTP_Load(t *testing.T) {
	data := []struct {
		name              string
		apps              int
		webContainers     int
		workerContainers  int
		routerContainers  int // 1 for apps with web containers, 0 for worker-only apps
		linesPerContainer int
	}{
		{"50 apps, 1 web, 1 worker", 50, 1, 1, 1, 10},
		{"100 apps, 2 web, 2 worker", 100, 2, 2, 1, 10},
		{"100 apps, 4 web, 4 worker", 100, 4, 4, 1, 10},
		{"200 apps, 4 web, 4 worker (future worst case)", 200, 4, 4, 1, 10},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			// Sizes: one sender per container.
			containersPerApp := d.webContainers + d.workerContainers + d.routerContainers
			senders := d.apps * containersPerApp
			want := senders * d.linesPerContainer

			validBody, err := json.Marshal(Request{Line: routerLine})
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Pipeline, as in main.go: entries → processor → repository.
			// done is closed once the processor has saved everything.
			var repo storage.LogRepository = &storage.MemoryLogRepository{}
			entries := make(chan logs.LogEntry, 10_000)
			p := ingest.Processor{
				NumberOfWorkers: 2,
				Repository:      repo,
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				p.Run(entries)
			}()

			// A real HTTP server in front of the handler.
			srv := httptest.NewServer(&Handler{Channel: entries})
			defer srv.Close()

			client := http.Client{
				Transport: &http.Transport{
					MaxIdleConns:        senders,
					MaxIdleConnsPerHost: senders,
					// Test-machine limit, not production behaviour: Windows refuses
					// a burst of more than ~200 simultaneous dials. In production
					// the service runs on Linux behind Scalingo's router.
					MaxConnsPerHost: 100,
				},
			}
			// send is one container: it posts its lines one after another.
			send := func() {
				for range d.linesPerContainer {
					res, err := client.Post(srv.URL+"/ingest", "application/json", strings.NewReader(string(validBody)))
					if err != nil {
						t.Errorf("http.Post failed: %v", err)
						return
					}
					res.Body.Close()
				}
			}

			// Run: all senders at once, then let the processor drain.
			start := time.Now()
			var wg sync.WaitGroup
			for range senders {
				wg.Go(send)
			}
			wg.Wait()
			close(entries)
			<-done
			elapsed := time.Since(start)

			// Check: every line was saved.
			f := storage.LogFilter{}
			result, err := repo.Find(f)
			if err != nil {
				t.Fatalf("Find(%+v) failed: %v", f, err)
			}
			if len(result) != want {
				t.Errorf("repository has %d entries, want %d", len(result), want)
			}
			t.Logf("%d senders, %d lines in %v", senders, want, elapsed)
		})
	}
}
