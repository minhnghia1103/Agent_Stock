package http

import (
	"encoding/json"
	"net/http"
)

// APIError is a small error catalog entry for JSON responses.
type APIError struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type errorBody struct {
	Error APIError `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, typ, message string) {
	writeJSON(w, status, errorBody{
		Error: APIError{
			Type:      typ,
			Message:   message,
			RequestID: RequestIDFromContext(r.Context()),
		},
	})
}
