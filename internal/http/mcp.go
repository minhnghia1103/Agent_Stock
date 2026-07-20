package http

import (
	"log/slog"
	"net/http"
)

func (s *Server) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	if s.mcp == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"path":    "",
			"servers": []any{},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":    s.mcp.Path(),
		"servers": s.mcp.Status(),
		"tools":   s.mcp.ToolNames(),
	})
}

func (s *Server) handleMCPReload(w http.ResponseWriter, r *http.Request) {
	if s.mcp == nil {
		writeError(w, r, http.StatusServiceUnavailable, "mcp_unavailable", "mcp manager not configured")
		return
	}
	if err := s.mcp.Reload(r.Context()); err != nil {
		slog.Error("mcp.reload_failed", "error", err)
		writeError(w, r, http.StatusInternalServerError, "mcp_reload_failed", err.Error())
		return
	}
	slog.Info("mcp.reloaded", "path", s.mcp.Path(), "tools", s.mcp.ToolNames())
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"path":    s.mcp.Path(),
		"servers": s.mcp.Status(),
		"tools":   s.mcp.ToolNames(),
	})
}
