package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
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
	fromTime, err := parseTimeQueryParam(r.URL.Query(), "from")
	if err != nil {
		slog.Warn("invalid query parameter", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	toTime, err := parseTimeQueryParam(r.URL.Query(), "to")
	if err != nil {
		slog.Warn("invalid query parameter", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	if found == nil {
		found = []logs.LogEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LogsResponse{Logs: found})
}

func parseTimeQueryParam(uv url.Values, paramValue string) (time.Time, error) {
	vUnparsed := uv.Get(paramValue)
	vParsed, err := time.Parse(time.RFC3339, vUnparsed)
	if vUnparsed != "" && err != nil {
		return vParsed, fmt.Errorf("invalid %q: %w", paramValue, err)
	}
	return vParsed, nil
}
