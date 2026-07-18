package http

import (
	"net/http"

	"agent_stock/internal/config"
)

// Server is the HTTP surface (health + chat stub).
type Server struct {
	cfg     *config.Config
	version string
	mux     *http.ServeMux
}

// New wires routes (composition root for HTTP).
func New(cfg *config.Config, version string) *Server {
	s := &Server{
		cfg:     cfg,
		version: version,
		mux:     http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /v1/chat", s.handleChat)
	return s
}

// Handler returns mux wrapped with middleware (request-id → recover).
func (s *Server) Handler() http.Handler {
	return recoverMiddleware(requestIDMiddleware(s.mux))
}
