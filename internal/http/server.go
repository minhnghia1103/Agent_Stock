package http

import (
	"net/http"

	"agent_stock/internal/config"
	"agent_stock/internal/security"
	"agent_stock/internal/session"
)

// Server is the HTTP surface (health + chat + stream + ws).
type Server struct {
	cfg      *config.Config
	version  string
	sessions *session.Service
	policy   *security.Policy
	mux      *http.ServeMux
}

// New wires routes (composition root for HTTP).
func New(cfg *config.Config, version string, sessions *session.Service, pol *security.Policy) *Server {
	s := &Server{
		cfg:      cfg,
		version:  version,
		sessions: sessions,
		policy:   pol,
		mux:      http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /v1/chat", s.handleChat)
	s.mux.HandleFunc("POST /v1/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("GET /ws", s.handleWS)
	return s
}

// Handler returns mux wrapped with middleware (request-id → recover).
func (s *Server) Handler() http.Handler {
	return recoverMiddleware(requestIDMiddleware(s.mux))
}
