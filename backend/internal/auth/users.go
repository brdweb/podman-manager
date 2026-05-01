package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrUsernameTaken      = errors.New("username already exists")
	ErrLastAdmin          = errors.New("cannot remove the last admin user")
	ErrSessionNotFound    = errors.New("session not found")
)

type User struct {
	ID        int64
	Username  string
	Role      Role
	CreatedAt time.Time
	LastLogin *time.Time
}

type CreateUserRequest struct {
	Username string
	Password string
	Role     Role
}

type UpdateUserRequest struct {
	Role *Role
}

func (s *Store) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
	username := strings.TrimSpace(req.Username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	role := req.Role
	if role == "" {
		role = RoleViewer
	}
	if !validRole(role) {
		return nil, fmt.Errorf("invalid role: %s", role)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO users (username, password_hash, role) VALUES (?, ?, ?)`, username, string(hash), string(role))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, ErrUsernameTaken
		}
		return nil, fmt.Errorf("creating user: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("reading created user id: %w", err)
	}
	return s.GetUser(ctx, id)
}

func (s *Store) GetUser(ctx context.Context, id int64) (*User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx, `SELECT id, username, role, created_at, last_login FROM users WHERE id = ?`, id))
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return s.scanUser(s.db.QueryRowContext(ctx, `SELECT id, username, role, created_at, last_login FROM users WHERE username = ?`, strings.TrimSpace(username)))
}

func (s *Store) ListUsers(ctx context.Context) ([]*User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, role, created_at, last_login FROM users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	users := make([]*User, 0)
	for rows.Next() {
		user, err := scanUserRows(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}
	return users, nil
}

func (s *Store) UpdateUser(ctx context.Context, id int64, req UpdateUserRequest) error {
	if req.Role == nil {
		return nil
	}
	if !validRole(*req.Role) {
		return fmt.Errorf("invalid role: %s", *req.Role)
	}
	if *req.Role != RoleAdmin {
		if err := s.ensureNotLastAdmin(ctx, id); err != nil {
			return err
		}
	}
	result, err := s.db.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, string(*req.Role), id)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}
	return ensureAffected(result, "user not found")
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	if err := s.ensureNotLastAdmin(ctx, id); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}
	return ensureAffected(result, "user not found")
}

func (s *Store) ChangePassword(ctx context.Context, id int64, currentPassword, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password is required")
	}
	var currentHash string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, id).Scan(&currentHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("reading password hash: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCredentials
	}
	return s.ResetPassword(ctx, id, newPassword)
}

func (s *Store) ResetPassword(ctx context.Context, id int64, newPassword string) error {
	if newPassword == "" {
		return fmt.Errorf("new password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	if err != nil {
		return fmt.Errorf("resetting password: %w", err)
	}
	return ensureAffected(result, "user not found")
}

func (s *Store) UpdateLastLogin(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE users SET last_login = ? WHERE id = ?`, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("updating last login: %w", err)
	}
	return ensureAffected(result, "user not found")
}

func (s *Store) VerifyPassword(ctx context.Context, username, password string) (*User, error) {
	var user User
	var passwordHash string
	var lastLogin sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, created_at, last_login FROM users WHERE username = ?`, strings.TrimSpace(username)).Scan(&user.ID, &user.Username, &passwordHash, &user.Role, &user.CreatedAt, &lastLogin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("reading user: %w", err)
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if err := s.UpdateLastLogin(ctx, user.ID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	user.LastLogin = &now
	return &user, nil
}

func (s *Store) UserCount(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("counting users: %w", err)
	}
	return count, nil
}

func (s *Store) EnsureUserWithPasswordHash(ctx context.Context, username, passwordHash string, role Role) error {
	username = strings.TrimSpace(username)
	if username == "" || passwordHash == "" {
		return nil
	}
	if role == "" {
		role = RoleAdmin
	}
	if !validRole(role) {
		return fmt.Errorf("invalid role: %s", role)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role)
		VALUES (?, ?, ?)
		ON CONFLICT(username) DO UPDATE SET password_hash = excluded.password_hash, role = excluded.role
	`, username, passwordHash, string(role))
	if err != nil {
		return fmt.Errorf("ensuring config auth user: %w", err)
	}
	return nil
}

func (s *Store) scanUser(row *sql.Row) (*User, error) {
	user, err := scanUserScanner(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

type userScanner interface {
	Scan(dest ...any) error
}

func scanUserRows(rows *sql.Rows) (*User, error) {
	user, err := scanUserScanner(rows)
	if err != nil {
		return nil, fmt.Errorf("scanning user: %w", err)
	}
	return user, nil
}

func scanUserScanner(scanner userScanner) (*User, error) {
	var user User
	var lastLogin sql.NullTime
	if err := scanner.Scan(&user.ID, &user.Username, &user.Role, &user.CreatedAt, &lastLogin); err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}
	return &user, nil
}

func (s *Store) ensureNotLastAdmin(ctx context.Context, id int64) error {
	var role Role
	if err := s.db.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("reading user role: %w", err)
	}
	if role != RoleAdmin {
		return nil
	}
	var adminCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ?`, string(RoleAdmin)).Scan(&adminCount); err != nil {
		return fmt.Errorf("counting admin users: %w", err)
	}
	if adminCount <= 1 {
		return ErrLastAdmin
	}
	return nil
}

func validRole(role Role) bool {
	switch role {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	default:
		return false
	}
}

func ensureAffected(result sql.Result, notFoundMessage string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		if notFoundMessage == "user not found" {
			return ErrUserNotFound
		}
		return errors.New(notFoundMessage)
	}
	return nil
}
