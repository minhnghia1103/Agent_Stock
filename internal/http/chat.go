package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

// ChatRequest is the Phase 1 stub body for POST /v1/chat.
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse is the stub reply (no LLM yet).
type ChatResponse struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	RequestID string `json:"request_id"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	const maxBody = 1 << 20 // 1MB
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

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = uuid.NewString()
	}

	reqID := RequestIDFromContext(r.Context())
	slog.Info("chat stub",
		"request_id", reqID,
		"session_id", sessionID,
		"message_len", len(req.Message),
	)

	writeJSON(w, http.StatusOK, ChatResponse{
		SessionID: sessionID,
		Reply:     "echo: " + req.Message,
		RequestID: reqID,
	})
}
