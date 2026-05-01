package enroll

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// EnrollmentToken represents a one-time token for agent enrollment.
type EnrollmentToken struct {
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
	AgentID   string // set after enrollment
}

// AgentCredential represents a persistent credential for an enrolled agent.
type AgentCredential struct {
	AgentID    string
	Hostname   string
	Credential string // persistent auth token
	CreatedAt  time.Time
	LastSeen   time.Time
	Active     bool
}

// Store manages enrollment tokens and agent credentials.
type Store struct {
	mu          sync.RWMutex
	tokens      map[string]*EnrollmentToken
	credentials map[string]*AgentCredential // agentID -> credential
	tokenTTL    time.Duration
}

// NewStore creates an in-memory enrollment store.
func NewStore(tokenTTL time.Duration) *Store {
	if tokenTTL <= 0 {
		tokenTTL = time.Hour
	}

	return &Store{
		tokens:      make(map[string]*EnrollmentToken),
		credentials: make(map[string]*AgentCredential),
		tokenTTL:    tokenTTL,
	}
}

// CreateToken creates a one-time enrollment token.
func (s *Store) CreateToken() (*EnrollmentToken, error) {
	token, err := s.generateToken()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	enrollmentToken := &EnrollmentToken{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(s.tokenTTL),
	}

	s.mu.Lock()
	s.tokens[token] = enrollmentToken
	s.mu.Unlock()

	return enrollmentToken, nil
}

// ValidateToken checks whether a token exists, has not expired, and is unused.
func (s *Store) ValidateToken(token string) (*EnrollmentToken, error) {
	s.mu.RLock()
	enrollmentToken, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("enrollment token not found")
	}
	if enrollmentToken.Used {
		return nil, fmt.Errorf("enrollment token already used")
	}
	if time.Now().After(enrollmentToken.ExpiresAt) {
		return nil, fmt.Errorf("enrollment token expired")
	}

	return enrollmentToken, nil
}

// ConsumeToken marks a token as used and records the enrolled agent ID.
func (s *Store) ConsumeToken(token string, agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	enrollmentToken, ok := s.tokens[token]
	if !ok {
		return fmt.Errorf("enrollment token not found")
	}
	if enrollmentToken.Used {
		return fmt.Errorf("enrollment token already used")
	}
	if time.Now().After(enrollmentToken.ExpiresAt) {
		return fmt.Errorf("enrollment token expired")
	}

	enrollmentToken.Used = true
	enrollmentToken.AgentID = agentID
	return nil
}

// RegisterCredential stores or updates a persistent agent credential.
func (s *Store) RegisterCredential(cred *AgentCredential) {
	if cred == nil || cred.AgentID == "" {
		return
	}
	stored := *cred

	s.mu.Lock()
	s.credentials[cred.AgentID] = &stored
	s.mu.Unlock()
}

// GetCredential returns a credential by agent ID.
func (s *Store) GetCredential(agentID string) (*AgentCredential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cred, ok := s.credentials[agentID]
	if !ok {
		return nil, false
	}
	clone := *cred
	return &clone, true
}

// GetCredentialByToken returns a credential by persistent credential token.
func (s *Store) GetCredentialByToken(token string) (*AgentCredential, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, cred := range s.credentials {
		if cred.Credential == token {
			clone := *cred
			return &clone, true
		}
	}
	return nil, false
}

// RevokeCredential disables a stored agent credential.
func (s *Store) RevokeCredential(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cred, ok := s.credentials[agentID]
	if !ok {
		return fmt.Errorf("agent credential not found")
	}
	cred.Active = false
	return nil
}

// ListCredentials returns all registered agent credentials.
func (s *Store) ListCredentials() []*AgentCredential {
	s.mu.RLock()
	defer s.mu.RUnlock()

	credentials := make([]*AgentCredential, 0, len(s.credentials))
	for _, cred := range s.credentials {
		clone := *cred
		credentials = append(credentials, &clone)
	}
	return credentials
}

// CleanupExpired removes expired unused tokens.
func (s *Store) CleanupExpired() {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, enrollmentToken := range s.tokens {
		if now.After(enrollmentToken.ExpiresAt) {
			delete(s.tokens, token)
		}
	}
}

func (s *Store) generateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
