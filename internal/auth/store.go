// Package auth manages provider credentials: API keys and OAuth tokens,
// persisted to a 0600 auth.json, plus the OAuth flows that obtain them.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type CredentialType string

const (
	TypeAPIKey CredentialType = "api"
	TypeOAuth  CredentialType = "oauth"
)

type Credential struct {
	Type      CredentialType `json:"type"`
	APIKey    string         `json:"apiKey,omitempty"`
	Access    string         `json:"access,omitempty"`
	Refresh   string         `json:"refresh,omitempty"`
	IDToken   string         `json:"idToken,omitempty"`
	ExpiresAt time.Time      `json:"expiresAt,omitempty"`
	AccountID string         `json:"accountId,omitempty"`
}

type Store struct {
	path              string
	mu                sync.Mutex
	creds             map[string]Credential
	platformCreds     map[string]PlatformCredential
	refreshMu         sync.Mutex
	refreshes         map[string]*platformRefreshCall
	platformRefresher PlatformRefresher
}

// DefaultPath is ~/.mr-reviewer/auth.json.
func DefaultPath() string {
	if p := strings.TrimSpace(os.Getenv("MR_REVIEWER_AUTH")); p != "" {
		return p
	}
	if root := strings.TrimSpace(os.Getenv("MR_REVIEWER_HOME")); root != "" {
		return filepath.Join(root, "auth.json")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "mr-reviewer", "auth.json")
	}
	root := filepath.Join(home, ".mr-reviewer")
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return filepath.Join(root, "auth.json")
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, creds: map[string]Credential{}, platformCreds: map[string]PlatformCredential{}, refreshes: map[string]*platformRefreshCall{}, platformRefresher: refreshPlatformOAuth}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if platforms, ok := raw["platforms"]; ok {
		if err := json.Unmarshal(platforms, &s.platformCreds); err != nil {
			return nil, err
		}
		delete(raw, "platforms")
	}
	for name, value := range raw {
		var credential Credential
		if err := json.Unmarshal(value, &credential); err != nil {
			return nil, err
		}
		s.creds[name] = credential
	}
	// Legacy platform records were global. Migrate PATs only to their public-cloud target.
	migrated := false
	for _, platform := range []string{"gitlab", "github"} {
		credential, ok := s.creds[platform]
		if !ok || credential.Type != TypeAPIKey || credential.APIKey == "" {
			continue
		}
		target, _ := PublicTarget(platform)
		s.platformCreds[target.Key()] = PlatformCredential{Type: PlatformPAT, Token: credential.APIKey}
		delete(s.creds, platform)
		migrated = true
	}
	if migrated {
		if err := s.save(context.Background(), s.creds, s.platformCreds); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func CanonicalProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "gemini" {
		return "google"
	}
	return provider
}

func canonicalProvider(provider string) string { return CanonicalProvider(provider) }

func (s *Store) Get(provider string) (Credential, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getLocked(provider)
}

func (s *Store) getLocked(provider string) (Credential, bool) {
	if canonicalProvider(provider) == "google" {
		if c, ok := s.creds["google"]; ok {
			return c, true
		}
		if c, ok := s.creds["gemini"]; ok {
			return c, true
		}
		return Credential{}, false
	}
	c, ok := s.creds[provider]
	return c, ok
}

func (s *Store) Set(provider string, c Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneCredentials(s.creds)
	if canonicalProvider(provider) == "google" {
		next["google"] = c
		delete(next, "gemini")
	} else {
		next[provider] = c
	}
	if err := s.save(context.Background(), next, s.platformCreds); err != nil {
		return err
	}
	s.creds = next
	return nil
}

func (s *Store) Delete(provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := cloneCredentials(s.creds)
	if canonicalProvider(provider) == "google" {
		delete(next, "google")
		delete(next, "gemini")
	} else {
		delete(next, provider)
	}
	if err := s.save(context.Background(), next, s.platformCreds); err != nil {
		return err
	}
	s.creds = next
	return nil
}

func (s *Store) Providers() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make(map[string]struct{}, len(s.creds))
	out := make([]string, 0, len(s.creds))
	for p := range s.creds {
		name := p
		if p == "gemini" {
			name = "google"
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func (s *Store) Path() string { return s.path }

func (s *Store) save(ctx context.Context, creds map[string]Credential, platforms map[string]PlatformCredential) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	payload := make(map[string]any, len(creds)+1)
	for name, credential := range creds {
		payload[name] = credential
	}
	if len(platforms) > 0 {
		payload["platforms"] = platforms
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".auth-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	if dirFile, err := os.Open(dir); err == nil {
		defer dirFile.Close()
		_ = dirFile.Sync()
	}
	return nil
}

func cloneCredentials(in map[string]Credential) map[string]Credential {
	out := make(map[string]Credential, len(in))
	for key, credential := range in {
		out[key] = credential
	}
	return out
}

func clonePlatformCredentials(in map[string]PlatformCredential) map[string]PlatformCredential {
	out := make(map[string]PlatformCredential, len(in))
	for key, credential := range in {
		out[key] = credential
	}
	return out
}
