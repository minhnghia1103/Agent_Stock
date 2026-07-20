package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"agent_stock/internal/auth"
	"agent_stock/internal/store"
)

func (s *Server) handleAgentsList(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if s.identities == nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "identity store not configured")
		return
	}
	list, err := s.identities.ListAgents(r.Context(), id.UserID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": list})
}

type createAgentRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (s *Server) handleAgentsCreate(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	if !id.CanWriteAgents() {
		writeError(w, r, http.StatusForbidden, "forbidden", "role cannot create agents")
		return
	}
	if s.identities == nil {
		writeError(w, r, http.StatusInternalServerError, "internal_error", "identity store not configured")
		return
	}
	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "slug is required")
		return
	}
	if req.Name == "" {
		req.Name = req.Slug
	}
	ag, err := s.identities.CreateAgent(r.Context(), store.Agent{
		TenantID:    id.TenantID,
		OwnerUserID: id.UserID,
		Name:        req.Name,
		Slug:        req.Slug,
	})
	if err != nil {
		slog.Error("create agent", "error", err)
		writeError(w, r, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ag)
}

func (s *Server) handleAgentGet(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	ref := r.PathValue("id")
	ag, err := s.identities.GetAgent(r.Context(), id.UserID, ref)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "not_found", "agent not found")
			return
		}
		writeError(w, r, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ag)
}
