package http

import (
	"net/http"

	"agent_stock/internal/auth"
	"agent_stock/internal/config"
	"agent_stock/internal/mcp"
	"agent_stock/internal/security"
	"agent_stock/internal/session"
	"agent_stock/internal/skills"
	"agent_stock/internal/store"
)

// Server is the HTTP surface.
type Server struct {
	cfg        *config.Config
	version    string
	sessions   *session.Service
	policy     *security.Policy
	mcp        *mcp.Manager
	skills     *skills.Registry
	auth       *auth.Authenticator
	identities store.IdentityStore
	mux        *http.ServeMux
}

// New wires routes (composition root for HTTP).
func New(
	cfg *config.Config,
	version string,
	sessions *session.Service,
	pol *security.Policy,
	mcpMgr *mcp.Manager,
	skillsReg *skills.Registry,
	authenticator *auth.Authenticator,
	identities store.IdentityStore,
) *Server {
	s := &Server{
		cfg:        cfg,
		version:    version,
		sessions:   sessions,
		policy:     pol,
		mcp:        mcpMgr,
		skills:     skillsReg,
		auth:       authenticator,
		identities: identities,
		mux:        http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /v1/chat", s.handleChat)
	s.mux.HandleFunc("POST /v1/chat/stream", s.handleChatStream)
	s.mux.HandleFunc("GET /ws", s.handleWS)
	s.mux.HandleFunc("GET /v1/mcp/servers", s.handleMCPServers)
	s.mux.HandleFunc("POST /v1/mcp/reload", s.handleMCPReload)
	s.mux.HandleFunc("GET /v1/skills", s.handleSkillsList)
	s.mux.HandleFunc("POST /v1/skills/reload", s.handleSkillsReload)
	s.mux.HandleFunc("GET /v1/agents", s.handleAgentsList)
	s.mux.HandleFunc("POST /v1/agents", s.handleAgentsCreate)
	s.mux.HandleFunc("GET /v1/agents/{id}", s.handleAgentGet)
	s.mux.HandleFunc("GET /v1/me", s.handleMe)
	return s
}

// Handler returns mux wrapped with middleware (request-id → recover → auth).
func (s *Server) Handler() http.Handler {
	return recoverMiddleware(requestIDMiddleware(s.authMiddleware(s.mux)))
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"tenant_id": id.TenantID,
		"user_id":   id.UserID,
		"email":     id.Email,
		"role":      id.Role,
	})
}
