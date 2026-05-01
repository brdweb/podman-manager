package enroll

import (
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentStoreLoadsAgentCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-credentials.json")
	createdAt := time.Now().UTC().Truncate(time.Second)
	lastSeen := createdAt.Add(time.Minute)

	store, err := NewPersistentStore(time.Hour, path)
	if err != nil {
		t.Fatalf("NewPersistentStore() error = %v", err)
	}
	if err := store.RegisterCredential(&AgentCredential{
		AgentID:    "agent_test",
		Hostname:   "test-host",
		Credential: "secret",
		CreatedAt:  createdAt,
		LastSeen:   lastSeen,
		Active:     true,
	}); err != nil {
		t.Fatalf("RegisterCredential() error = %v", err)
	}

	loaded, err := NewPersistentStore(time.Hour, path)
	if err != nil {
		t.Fatalf("NewPersistentStore() reload error = %v", err)
	}

	cred, ok := loaded.GetCredential("agent_test")
	if !ok {
		t.Fatal("GetCredential() did not find persisted credential")
	}
	if cred.Hostname != "test-host" || cred.Credential != "secret" || !cred.Active {
		t.Fatalf("GetCredential() = %#v, want persisted credential", cred)
	}
}
