package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"agent_stock/internal/progress"
	"agent_stock/internal/provider"
)

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	const maxBody = 1 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	if req.Message == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "message is required")
		return
	}
	if s.sessions == nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "session service not configured")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "streaming unsupported")
		return
	}

	reqID := RequestIDFromContext(r.Context())
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	writeSSE := func(ev progress.Event) {
		raw, err := json.Marshal(ev)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, raw)
		flusher.Flush()
	}

	_, err := s.sessions.ChatTurnWithEvents(r.Context(), req.SessionID, req.Message, req.AgentID, writeSSE)
	if err != nil {
		slog.Error("chat stream failed", "request_id", reqID, "error", err)
		msg := err.Error()
		var httpErr *provider.HTTPError
		if errors.As(err, &httpErr) {
			msg = httpErr.Error()
		}
		writeSSE(progress.Event{
			Type:    progress.EventError,
			Payload: map[string]any{"message": msg, "request_id": reqID},
		})
	}
}
