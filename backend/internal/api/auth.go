package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	authpkg "github.com/brdweb/podman-manager/internal/auth"
)

const sessionCookieName = "podman_manager_session"

type sessionResponse struct {
	Enabled       bool   `json:"enabled"`
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
	Role          string `json:"role,omitempty"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func withAuth(server *Server) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authConfig := server.authConfig()
			if r.Method == http.MethodOptions || isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if !authConfig.Enabled {
				ctx := context.WithValue(r.Context(), sessionKey, &authpkg.Session{Role: authpkg.RoleAdmin})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			sess, err := server.authStore.GetSession(r.Context(), cookie.Value)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			ctx := context.WithValue(r.Context(), sessionKey, sess)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isPublicPath(path string) bool {
	switch path {
	case "/api/health", "/api/version", "/api/auth/session", "/api/auth/login", "/api/agent/install.sh":
		return true
	default:
		return false
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	auth := s.authConfig()
	resp := sessionResponse{}

	if !auth.Enabled {
		resp.Authenticated = true
		resp.Role = string(authpkg.RoleAdmin)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Enabled = true
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if current, err := s.authStore.GetSession(r.Context(), cookie.Value); err == nil {
		resp.Authenticated = true
		resp.Username = current.Username
		resp.Role = string(current.Role)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	auth := s.authConfig()
	if !auth.Enabled {
		writeJSON(w, http.StatusOK, sessionResponse{Enabled: false, Authenticated: true, Role: string(authpkg.RoleAdmin)})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login payload")
		return
	}

	user, err := s.authStore.VerifyPassword(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	session, err := s.authStore.CreateSession(r.Context(), user.ID, auth.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  time.Now().Add(auth.SessionTTL),
	})

	writeJSON(w, http.StatusOK, sessionResponse{
		Enabled:       true,
		Authenticated: true,
		Username:      user.Username,
		Role:          string(user.Role),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := s.authStore.DeleteSession(r.Context(), cookie.Value); err != nil {
			s.logger.Warn("failed to delete auth session", "error", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
