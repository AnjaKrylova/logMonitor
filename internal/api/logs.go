package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

type LogsHandler struct {
	Repository storage.LogRepository
}

type LogsResponse struct {
	Logs []logs.LogEntry `json:"logs"`
}

func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	fromTime, err := time.Parse(time.RFC3339, from)
	if from != "" && err != nil {
		slog.Warn("failed to parse 'from' parameter", "error", err)
		http.Error(w, "failed to parse 'from' parameter", http.StatusBadRequest)
		return
	}
	toTime, err := time.Parse(time.RFC3339, to)
	if to != "" && err != nil {
		slog.Warn("failed to parse 'to' parameter", "error", err)
		http.Error(w, "failed to parse 'to' parameter", http.StatusBadRequest)
		return
	}
	filter := storage.LogFilter{
		App:  app,
		From: fromTime,
		To:   toTime,
	}
	found, err := h.Repository.Find(filter)
	if err != nil {
		slog.Error("failed to find logs", "error", err)
		http.Error(w, "failed to find logs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LogsResponse{Logs: found})
}
