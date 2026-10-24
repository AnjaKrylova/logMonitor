package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"logmonitor/internal/api"
	"logmonitor/internal/ingest"
	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

type healthResponse struct {
	Status string `json:"status"`
}

type versionResponse struct {
	Version string `json:"version"`
}

func main() {
	mux := http.NewServeMux()
	s := http.Server{
		Addr:         ":8080",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
	})

	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(versionResponse{Version: "0.1.0"})
	})

	var repo storage.LogRepository = &storage.MemoryLogRepository{}
	ch := make(chan logs.LogEntry)
	p := ingest.Processor{
		NumberOfWorkers: 2,
		Repository:      repo,
	}
	go p.Run(ch)
	h := &api.Handler{Channel: ch}

	mux.Handle("POST /ingest", h)

	err := s.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}

}
