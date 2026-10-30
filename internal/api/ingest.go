package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"logmonitor/internal/logs"
)

type Request struct {
	Line string `json:"line"`
}

type Handler struct {
	Channel chan<- logs.LogEntry
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body Request
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		slog.Warn("failed to decode request body", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Line == "" {
		slog.Warn("line is required")
		http.Error(w, "line is required", http.StatusBadRequest)
		return
	}
	entry, err := logs.ParseLog(body.Line, time.Now())
	if err != nil {
		slog.Warn("failed to parse log line", "error", err)
	}

	select {
	case h.Channel <- entry:
	default:
		slog.Error("queue is full, waiting")
		h.Channel <- entry
	}

	w.WriteHeader(http.StatusAccepted)
}
