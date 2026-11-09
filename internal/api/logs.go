package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"logmonitor/internal/logs"
	"logmonitor/internal/storage"
)

const (
	defaultLimit = 50
	maxLimit     = 500
)

type LogsHandler struct {
	Repository storage.LogRepository
}

type LogsResponse struct {
	Logs []logs.LogEntry `json:"logs"`
}

func (h *LogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	containerType := r.URL.Query().Get("container_type")
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
	limit, err := parseIntQueryParam(r.URL.Query(), "limit", defaultLimit)
	if err != nil {
		slog.Warn("invalid query parameter", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if limit <= 0 || limit > maxLimit {
		err := fmt.Errorf("limit must be between 1 and %d, got %d", maxLimit, limit)
		slog.Warn("invalid query parameter", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	filter := storage.LogFilter{
		App:           app,
		From:          fromTime,
		To:            toTime,
		ContainerType: containerType,
		Limit:         limit,
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

func parseIntQueryParam(uv url.Values, paramValue string, defaultV int) (int, error) {
	vUnparsed := uv.Get(paramValue)
	if vUnparsed == "" {
		return defaultV, nil
	}
	vParsed, err := strconv.Atoi(vUnparsed)
	if err != nil {
		return vParsed, fmt.Errorf("invalid %q: %w", paramValue, err)
	}
	return vParsed, nil
}
