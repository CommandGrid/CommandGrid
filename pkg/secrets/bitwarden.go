package secrets

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// BitwardenStore is a read-only store that resolves secrets from Bitwarden CLI.
// It expects "bw" to be installed and authenticated/unlocked.
type BitwardenStore struct {
	session string
}

// NewBitwardenStore creates a read-only Bitwarden-backed secret store.
func NewBitwardenStore() (*BitwardenStore, error) {
	if _, err := exec.LookPath("bw"); err != nil {
		return nil, fmt.Errorf("bitwarden CLI not found: %w", err)
	}
	return &BitwardenStore{
		session: strings.TrimSpace(os.Getenv("BW_SESSION")),
	}, nil
}

// Get resolves a secret from Bitwarden by item name.
// It searches items and prefers an exact name match; value resolution order:
// login.password, then notes.
func (s *BitwardenStore) Get(name string) (string, error) {
	args := []string{"list", "items", "--search", name}
	if s.session != "" {
		args = append(args, "--session", s.session)
	}
	out, err := exec.Command("bw", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("bitwarden list items failed: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	raw := strings.TrimSpace(string(out))
	if looksLikeBitwardenUnlockPrompt(raw) {
		return "", fmt.Errorf(
			"bitwarden vault is locked; run `export BW_SESSION=\"$(bw unlock --raw)\"` in this shell and retry",
		)
	}

	type bwItem struct {
		Name  string `json:"name"`
		Notes string `json:"notes"`
		Login struct {
			Password string `json:"password"`
		} `json:"login"`
	}
	var items []bwItem
	if err := json.Unmarshal(out, &items); err != nil {
		return "", fmt.Errorf("parsing bitwarden response: %w", err)
	}
	if len(items) == 0 {
		return "", fmt.Errorf("secret not found in bitwarden: %s", name)
	}

	var selected *bwItem
	for i := range items {
		if items[i].Name == name {
			selected = &items[i]
			break
		}
	}
	if selected == nil {
		selected = &items[0]
	}

	if selected.Login.Password != "" {
		return selected.Login.Password, nil
	}
	if strings.TrimSpace(selected.Notes) != "" {
		return strings.TrimSpace(selected.Notes), nil
	}
	return "", fmt.Errorf("bitwarden item %q has no login.password or notes value", selected.Name)
}

func looksLikeBitwardenUnlockPrompt(s string) bool {
	if s == "" {
		return false
	}
	l := strings.ToLower(s)
	return strings.Contains(l, "master password") ||
		strings.Contains(l, "vault is locked") ||
		strings.Contains(l, "you are not logged in")
}

// Set is unsupported for BitwardenStore.
func (s *BitwardenStore) Set(name, value string) error {
	return fmt.Errorf("bitwarden store is read-only")
}

// Delete is unsupported for BitwardenStore.
func (s *BitwardenStore) Delete(name string) error {
	return fmt.Errorf("bitwarden store is read-only")
}

// List is unsupported for BitwardenStore to avoid unexpected broad vault reads.
func (s *BitwardenStore) List() ([]string, error) {
	return nil, fmt.Errorf("bitwarden store does not support list")
}
