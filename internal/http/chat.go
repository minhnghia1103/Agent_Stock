package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"agent_stock/internal/provider"
)

// ChatRequest is the body for POST /v1/chat.
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id,omitempty"`
}

// ChatResponse is the chat reply.
type ChatResponse struct {
	SessionID    string          `json:"session_id"`
	Reply        string          `json:"reply"`
	RequestID    string          `json:"request_id"`
	MessageCount int             `json:"message_count"`
	Model        string          `json:"model,omitempty"`
	Usage        *provider.Usage `json:"usage,omitempty"`
	Iterations   int             `json:"iterations,omitempty"`
	ToolCalls    int             `json:"tool_calls,omitempty"`
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
	if s.sessions == nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "session service not configured")
		return
	}

	reqID := RequestIDFromContext(r.Context())
	result, err := s.sessions.ChatTurn(r.Context(), req.SessionID, req.Message)
	if err != nil {
		slog.Error("chat turn failed",
			"request_id", reqID,
			"session_id", req.SessionID,
			"error", err,
		)
		status := http.StatusInternalServerError
		typ := "internal_error"
		msg := "chat turn failed"
		var httpErr *provider.HTTPError
		if errors.As(err, &httpErr) {
			status = http.StatusBadGateway
			typ = "llm_error"
			msg = httpErr.Error()
		}
		writeError(w, r, status, typ, msg)
		return
	}

	slog.Info("chat turn",
		"request_id", reqID,
		"session_id", result.SessionID,
		"message_count", result.MessageCount,
		"model", result.Model,
		"iterations", result.Iterations,
		"tool_calls", result.ToolCalls,
	)

	writeJSON(w, http.StatusOK, ChatResponse{
		SessionID:    result.SessionID,
		Reply:        result.Reply,
		RequestID:    reqID,
		MessageCount: result.MessageCount,
		Model:        result.Model,
		Usage:        result.Usage,
		Iterations:   result.Iterations,
		ToolCalls:    result.ToolCalls,
	})
}
