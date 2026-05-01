package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/brdweb/podman-manager/internal/auth"
)

type userCreateRequest struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Role     auth.Role `json:"role"`
}

type userUpdateRequest struct {
	Role *auth.Role `json:"role"`
}

type passwordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type passwordResetRequest struct {
	NewPassword string `json:"new_password"`
}

type userResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Role      auth.Role `json:"role"`
	CreatedAt string    `json:"created_at"`
	LastLogin *string   `json:"last_login,omitempty"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	users, err := s.authStore.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	resp := make([]userResponse, 0, len(users))
	for _, user := range users {
		resp = append(resp, toUserResponse(user))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}

	var req userCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user payload")
		return
	}

	username := strings.TrimSpace(req.Username)
	if err := validateUsername(username); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validatePassword(req.Password); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !req.Role.Valid() {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}

	if _, err := s.authStore.GetUserByUsername(r.Context(), username); err == nil {
		writeError(w, http.StatusConflict, "username already exists")
		return
	} else if !isNotFoundError(err) {
		writeError(w, http.StatusInternalServerError, "failed to check username")
		return
	}

	user, err := s.authStore.CreateUser(r.Context(), auth.CreateUserRequest{
		Username: username,
		Password: req.Password,
		Role:     req.Role,
	})
	if isConflictError(err) {
		writeError(w, http.StatusConflict, "username already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	writeJSON(w, http.StatusCreated, toUserResponse(user))
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}
	current, authenticated := currentUser(r)
	if !authenticated || (current.Role != auth.RoleAdmin && current.UserID != id) {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return
	}

	user, err := s.authStore.GetUser(r.Context(), id)
	if isNotFoundError(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}

	var req userUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid user payload")
		return
	}
	if req.Role == nil || !req.Role.Valid() {
		writeError(w, http.StatusBadRequest, "invalid role")
		return
	}

	if err := s.authStore.UpdateUser(r.Context(), id, auth.UpdateUserRequest{Role: req.Role}); isNotFoundError(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if isLastAdminError(err) {
		writeError(w, http.StatusBadRequest, "cannot remove the last admin")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	user, err := s.authStore.GetUser(r.Context(), id)
	if isNotFoundError(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}
	current, _ := currentUser(r)
	if current.UserID == id {
		writeError(w, http.StatusForbidden, "cannot delete current user")
		return
	}

	if err := s.authStore.DeleteUser(r.Context(), id); isNotFoundError(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if isLastAdminError(err) {
		writeError(w, http.StatusBadRequest, "cannot delete the last admin")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, ok := parseUserID(w, r)
	if !ok {
		return
	}

	var req passwordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid password payload")
		return
	}
	if err := validatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.authStore.ResetPassword(r.Context(), id, req.NewPassword); isNotFoundError(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	current, ok := currentUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req passwordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid password payload")
		return
	}
	if strings.TrimSpace(req.CurrentPassword) == "" {
		writeError(w, http.StatusBadRequest, "current password is required")
		return
	}
	if err := validatePassword(req.NewPassword); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := s.authStore.ChangePassword(r.Context(), current.UserID, req.CurrentPassword, req.NewPassword); errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnprocessableEntity, "current password incorrect")
		return
	} else if isNotFoundError(err) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to change password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	current, ok := currentUser(r)
	if !ok || current.Role != auth.RoleAdmin {
		writeError(w, http.StatusForbidden, "insufficient permissions")
		return false
	}
	return true
}

func parseUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return 0, false
	}
	return id, true
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 64 {
		return errors.New("username must be between 3 and 64 characters")
	}
	for _, r := range username {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return errors.New("username may contain only letters, numbers, underscores, and hyphens")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func toUserResponse(user *auth.User) userResponse {
	resp := userResponse{
		ID:        user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
	if user.LastLogin != nil {
		lastLogin := user.LastLogin.Format(time.RFC3339)
		resp.LastLogin = &lastLogin
	}
	return resp
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}

func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "constraint") || strings.Contains(message, "already exists")
}

func isLastAdminError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "last admin")
}
