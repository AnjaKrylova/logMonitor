package api

import (
	"encoding/json"
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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Line == "" {
		http.Error(w, "line is required", http.StatusBadRequest)
		return
	}
	entry, _ := logs.ParseLog(body.Line, time.Now())
	h.Channel <- entry
	w.WriteHeader(http.StatusAccepted)
}
