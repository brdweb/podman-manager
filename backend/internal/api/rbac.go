package api

import (
	"net/http"

	"github.com/brdweb/podman-manager/internal/auth"
)

// contextKey is the type for context values.
type contextKey string

const (
	sessionKey contextKey = "session"
)

// withRBAC is a middleware that checks the user's role against the required roles.
// If the user doesn't have a required role, it returns 403 Forbidden.
func withRBAC(requiredRoles ...auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sess, ok := currentUser(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			if len(requiredRoles) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			for _, role := range requiredRoles {
				if sess.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			writeError(w, http.StatusForbidden, "insufficient permissions")
		})
	}
}

// currentUser extracts the session from the request context.
func currentUser(r *http.Request) (*auth.Session, bool) {
	sess, ok := r.Context().Value(sessionKey).(*auth.Session)
	return sess, ok
}
