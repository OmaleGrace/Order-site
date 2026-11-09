package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

var startedAt = time.Now()

// Health answers /healthz for uptime monitors and the host's health check. It says only whether
// the site is up and whether the database answers: nothing private is shown.
//
//	200 {"status":"ok"}        the site and its database are fine
//	503 {"status":"degraded"}  the site is running but the database did not answer in time
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	begin := time.Now()
	err := h.DB.PingContext(ctx)
	status, label := http.StatusOK, "ok"
	if err != nil {
		status, label = http.StatusServiceUnavailable, "degraded"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":     label,
		"database":   err == nil,
		"db_ms":      time.Since(begin).Milliseconds(),
		"uptime_sec": int64(time.Since(startedAt).Seconds()),
	})
}
