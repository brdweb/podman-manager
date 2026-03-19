package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "podman_manager_session"

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]session
}

type session struct {
	Username string
	Expires  time.Time
}

type sessionResponse struct {
	Enabled       bool   `json:"enabled"`
	Authenticated bool   `json:"authenticated"`
	Username      string `json:"username,omitempty"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func newSessionStore() *sessionStore {
	return &sessionStore{
		sessions: make(map[string]session),
	}
}

func (s *sessionStore) create(username string, ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}

	token := hex.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = session{
		Username: username,
		Expires:  time.Now().Add(ttl),
	}
	return token, nil
}

func (s *sessionStore) get(token string) (session, bool) {
	s.mu.RLock()
	current, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return session{}, false
	}
	if time.Now().After(current.Expires) {
		s.delete(token)
		return session{}, false
	}
	return current, true
}

func (s *sessionStore) delete(token string) {
	if token == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func withAuth(server *Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions || !server.authConfig().Enabled || isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		if _, ok := server.sessions.get(cookie.Value); !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isPublicPath(path string) bool {
	switch path {
	case "/api/health", "/api/auth/session", "/api/auth/login":
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
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.Enabled = true
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	if current, ok := s.sessions.get(cookie.Value); ok {
		resp.Authenticated = true
		resp.Username = current.Username
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	auth := s.authConfig()
	if !auth.Enabled {
		writeJSON(w, http.StatusOK, sessionResponse{Enabled: false, Authenticated: true})
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login payload")
		return
	}

	if strings.TrimSpace(req.Username) != auth.Username {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(auth.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	token, err := s.sessions.create(auth.Username, auth.SessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		Expires:  time.Now().Add(auth.SessionTTL),
	})

	writeJSON(w, http.StatusOK, sessionResponse{
		Enabled:       true,
		Authenticated: true,
		Username:      auth.Username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessions.delete(cookie.Value)
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
