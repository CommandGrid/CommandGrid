package secrets

import (
	"strings"
	"testing"
)

func TestEnvStore_GetFromEnv(t *testing.T) {
	t.Setenv("SECRET_MY_KEY", "env-value")

	store, err := NewEnvStore("", "SECRET_")
	if err != nil {
		t.Fatalf("NewEnvStore() error = %v", err)
	}

	got, err := store.Get("my_key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "env-value" {
		t.Errorf("Get() = %q, want %q", got, "env-value")
	}
}

func TestEnvStore_GetNotFound(t *testing.T) {
	store, err := NewEnvStore("", "SECRET_")
	if err != nil {
		t.Fatalf("NewEnvStore() error = %v", err)
	}

	_, err = store.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for nonexistent secret")
	}
}

func TestEnvStore_SetAndGet(t *testing.T) {
	store, err := NewEnvStore("", "SECRET_")
	if err != nil {
		t.Fatalf("NewEnvStore() error = %v", err)
	}

	_ = store.Set("runtime_key", "runtime-value")

	got, err := store.Get("runtime_key")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "runtime-value" {
		t.Errorf("Get() = %q, want %q", got, "runtime-value")
	}
}

func TestGenerateSessionToken(t *testing.T) {
	token, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken() error = %v", err)
	}

	if !strings.HasPrefix(token, "session-") {
		t.Errorf("token %q does not have session- prefix", token)
	}

	// session- (8) + 64 hex chars = 72
	if len(token) != 72 {
		t.Errorf("token length = %d, want 72", len(token))
	}

	// Two tokens should be different.
	token2, _ := GenerateSessionToken()
	if token == token2 {
		t.Error("two generated tokens are identical")
	}
}

// TestStoreInterface verifies implementations satisfy the Store interface.
func TestStoreInterface(t *testing.T) {
	envStore, err := NewEnvStore("", "")
	if err != nil {
		t.Fatal(err)
	}
	var _ Store = envStore
}
