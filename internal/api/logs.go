package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

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
	found, err := h.Repository.Find(storage.LogFilter{})
	if err != nil {
		slog.Error("failed to find logs", "error", err)
		http.Error(w, "failed to find logs", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LogsResponse{Logs: found})
}
