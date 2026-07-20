package http

import (
	"net/http"

	"agent_stock/internal/config"
	"agent_stock/internal/mcp"
	"agent_stock/internal/security"
	"agent_stock/internal/session"
	"agent_stock/internal/skills"
)

// Server is the HTTP surface (health + chat + stream + ws + mcp + skills).
type Server struct {
	cfg      *config.Config
	version  string
	sessions *session.Service
	policy   *security.Policy
	mcp      *mcp.Manager
	skills   *skills.Registry
	mux      *http.ServeMux
}

// New wires routes (composition root for HTTP).
func New(cfg *config.Config, version string, sessions *session.Service, pol *security.Policy, mcpMgr *mcp.Manager, skillsReg *skills.Registry) *Server {
	s := &Server{
		cfg:      cfg,
		version:  version,
		sessions: sessions,
		policy:   pol,
		mcp:      mcpMgr,
		skills:   skillsReg,
		mux:      http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /v1/chat", s.handleChat)
	s.mux.HandleFunc("POST /v1/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("GET /ws", s.handleWS)
	s.mux.HandleFunc("GET /v1/mcp/servers", s.handleMCPServers)
	s.mux.HandleFunc("POST /v1/mcp/reload", s.handleMCPReload)
	s.mux.HandleFunc("GET /v1/skills", s.handleSkillsList)
	s.mux.HandleFunc("POST /v1/skills/reload", s.handleSkillsReload)
	return s
}

// Handler returns mux wrapped with middleware (request-id → recover).
func (s *Server) Handler() http.Handler {
	return recoverMiddleware(requestIDMiddleware(s.mux))
}
