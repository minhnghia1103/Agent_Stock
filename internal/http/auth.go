package http

import (
	"net/http"
	"strings"

	"agent_stock/internal/auth"
	"agent_stock/internal/store"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if s.auth == nil {
			writeError(w, r, http.StatusInternalServerError, "auth_unavailable", "authenticator not configured")
			return
		}
		raw := bearerToken(r)
		if raw == "" {
			raw = r.URL.Query().Get("api_key")
		}
		if raw == "" {
			writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing Authorization Bearer API key")
			return
		}
		id, err := s.auth.AuthenticateRaw(r.Context(), raw)
		if err != nil {
			status := http.StatusUnauthorized
			if err == store.ErrForbidden {
				status = http.StatusForbidden
			}
			writeError(w, r, status, "unauthorized", "invalid API key")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithIdentity(r.Context(), *id)))
	})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const p = "Bearer "
	if strings.HasPrefix(h, p) {
		return strings.TrimSpace(h[len(p):])
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}
