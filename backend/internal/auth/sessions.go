package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

type Session struct {
	Token     string
	UserID    int64
	Username  string
	Role      Role
	ExpiresAt time.Time
}

func (s *Store) CreateSession(ctx context.Context, userID int64, ttl time.Duration) (*Session, error) {
	token, err := s.generateToken()
	if err != nil {
		return nil, err
	}
	expiresAt := time.Now().UTC().Add(ttl)
	if _, err := s.db.ExecContext(ctx, `INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`, token, userID, expiresAt); err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return s.GetSession(ctx, token)
}

func (s *Store) GetSession(ctx context.Context, token string) (*Session, error) {
	var session Session
	err := s.db.QueryRowContext(ctx, `
		SELECT sessions.token, sessions.user_id, users.username, users.role, sessions.expires_at
		FROM sessions
		JOIN users ON users.id = sessions.user_id
		WHERE sessions.token = ?
	`, token).Scan(&session.Token, &session.UserID, &session.Username, &session.Role, &session.ExpiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("reading session: %w", err)
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		if err := s.DeleteSession(ctx, token); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	return &session, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}
	return nil
}

func (s *Store) CleanupExpiredSessions(ctx context.Context) (int, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("cleaning up expired sessions: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("checking expired session cleanup count: %w", err)
	}
	return int(rows), nil
}

func (s *Store) generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
