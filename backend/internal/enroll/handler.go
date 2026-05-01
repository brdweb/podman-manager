package enroll

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// Handler provides HTTP endpoints for enrollment management.
type Handler struct {
	store *Store
}

// NewHandler creates an enrollment HTTP handler.
func NewHandler(store *Store) *Handler {
	if store == nil {
		store = NewStore(0)
	}
	return &Handler{store: store}
}

// CreateToken handles POST /api/admin/agents/enroll.
func (h *Handler) CreateToken(w http.ResponseWriter, r *http.Request) {
	h.handleCreateToken(w, r)
}

// ListAgents handles GET /api/admin/agents.
func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	h.handleListAgents(w, r)
}

// RevokeAgent handles POST /api/admin/agents/{id}/revoke.
func (h *Handler) RevokeAgent(w http.ResponseWriter, r *http.Request) {
	h.handleRevokeAgent(w, r)
}

// InstallScript handles GET /api/agent/install.sh.
func (h *Handler) InstallScript(w http.ResponseWriter, r *http.Request) {
	h.handleInstallScript(w, r)
}

// handleCreateToken handles POST /api/admin/agents/enroll.
func (h *Handler) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	token, err := h.store.CreateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create enrollment token")
		return
	}

	managerURL := managerGRPCAddress(r)
	writeJSON(w, http.StatusOK, map[string]string{
		"token":           token.Token,
		"expires_at":      token.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		"install_command": "curl -sL http://" + r.Host + "/api/agent/install.sh | bash -s -- --manager-url " + managerURL + " --token " + token.Token,
	})
}

// handleListAgents handles GET /api/admin/agents.
func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	_ = r

	credentials := h.store.ListCredentials()
	agents := make([]agentResponse, 0, len(credentials))
	for _, cred := range credentials {
		agents = append(agents, agentResponse{
			AgentID:   cred.AgentID,
			Hostname:  cred.Hostname,
			CreatedAt: cred.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastSeen:  cred.LastSeen.Format("2006-01-02T15:04:05Z07:00"),
			Active:    cred.Active,
		})
	}
	writeJSON(w, http.StatusOK, agents)
}

// handleRevokeAgent handles POST /api/admin/agents/{id}/revoke.
func (h *Handler) handleRevokeAgent(w http.ResponseWriter, r *http.Request) {
	agentID := strings.TrimSpace(r.PathValue("id"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "agent id is required")
		return
	}

	if err := h.store.RevokeCredential(agentID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Agent revoked"})
}

// handleInstallScript handles GET /api/agent/install.sh.
func (h *Handler) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	script := strings.ReplaceAll(installScript, "__MANAGER_URL__", managerGRPCAddress(r))
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(script))
}

type agentResponse struct {
	AgentID   string `json:"agent_id"`
	Hostname  string `json:"hostname"`
	CreatedAt string `json:"created_at"`
	LastSeen  string `json:"last_seen"`
	Active    bool   `json:"active"`
}

func managerGRPCAddress(r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return "localhost:18735"
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		if h == "" {
			h = "localhost"
		}
		return net.JoinHostPort(h, "18735")
	}
	return net.JoinHostPort(host, "18735")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
