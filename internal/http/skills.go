package http

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleSkillsList(w http.ResponseWriter, r *http.Request) {
	if s.skills == nil {
		writeJSON(w, http.StatusOK, map[string]any{"skills": []any{}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": s.skills.List()})
}

func (s *Server) handleSkillsReload(w http.ResponseWriter, r *http.Request) {
	if s.skills == nil {
		writeError(w, r, http.StatusServiceUnavailable, "skills_unavailable", "skills registry not configured")
		return
	}
	if err := s.skills.Reload(); err != nil {
		slog.Error("skills.reload_failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "skills_reload_failed", err.Error())
		return
	}
	slog.Info("skills.reloaded", "count", len(s.skills.List()))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"skills": s.skills.List(),
	})
}
