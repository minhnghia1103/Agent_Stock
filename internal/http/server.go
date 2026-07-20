package http

import (
	"net/http"

	"agent_stock/internal/config"
	"agent_stock/internal/mcp"
	"agent_stock/internal/security"
	"agent_stock/internal/session"
)

// Server is the HTTP surface (health + chat + stream + ws + mcp).
type Server struct {
	cfg      *config.Config
	version  string
	sessions *session.Service
	policy   *security.Policy
	mcp      *mcp.Manager
	mux      *http.ServeMux
}

// New wires routes (composition root for HTTP).
func New(cfg *config.Config, version string, sessions *session.Service, pol *security.Policy, mcpMgr *mcp.Manager) *Server {
	s := &Server{
		cfg:      cfg,
		version:  version,
		sessions: sessions,
		policy:   pol,
		mcp:      mcpMgr,
		mux:      http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /v1/chat", s.handleChat)
	s.mux.HandleFunc("POST /v1/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("GET /ws", s.handleWS)
	s.mux.HandleFunc("GET /v1/mcp/servers", s.handleMCPServers)
	s.mux.HandleFunc("POST /v1/mcp/reload", s.handleMCPReload)
	return s
}

// Handler returns mux wrapped with middleware (request-id → recover).
func (s *Server) Handler() http.Handler {
	return recoverMiddleware(requestIDMiddleware(s.mux))
}
