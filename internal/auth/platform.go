package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type PlatformTarget struct {
	Platform string `json:"platform"`
	Origin   string `json:"origin"`
	APIBase  string `json:"apiBase"`
}

func NewPlatformTarget(platform, origin, apiBase string) (PlatformTarget, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform != "gitlab" && platform != "github" {
		return PlatformTarget{}, fmt.Errorf("unsupported platform %q", platform)
	}
	normalize := func(raw string, originOnly bool) (string, error) {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (originOnly && u.Path != "") {
			return "", errors.New("invalid platform target URL")
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return "", errors.New("platform target URL must use HTTP or HTTPS")
		}
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
		u.Path = strings.TrimRight(u.Path, "/")
		return u.String(), nil
	}
	o, err := normalize(origin, true)
	if err != nil {
		return PlatformTarget{}, err
	}
	a, err := normalize(apiBase, false)
	if err != nil {
		return PlatformTarget{}, err
	}
	return PlatformTarget{Platform: platform, Origin: o, APIBase: a}, nil
}

func PublicTarget(platform string) (PlatformTarget, error) {
	switch strings.ToLower(platform) {
	case "gitlab":
		return NewPlatformTarget("gitlab", "https://gitlab.com", "https://gitlab.com/api/v4")
	case "github":
		return NewPlatformTarget("github", "https://github.com", "https://api.github.com")
	default:
		return PlatformTarget{}, fmt.Errorf("unsupported platform %q", platform)
	}
}

func (t PlatformTarget) Key() string { return t.Platform + "|" + t.Origin + "|" + t.APIBase }

func (t PlatformTarget) IsPublicCloud() bool {
	public, err := PublicTarget(t.Platform)
	return err == nil && t == public
}

type PlatformCredentialType string

const (
	PlatformPAT   PlatformCredentialType = "pat"
	PlatformOAuth PlatformCredentialType = "oauth"
)

type PlatformCredential struct {
	Type      PlatformCredentialType `json:"type"`
	Token     string                 `json:"token,omitempty"`
	Refresh   string                 `json:"refresh,omitempty"`
	ExpiresAt time.Time              `json:"expiresAt,omitempty"`
	ClientID  string                 `json:"clientId,omitempty"`
}

var ErrPlatformLoginRequired = errors.New("platform credentials require re-login")

type PlatformRefresher func(context.Context, PlatformTarget, PlatformCredential) (PlatformCredential, error)

func (s *Store) SetPlatformRefresher(refresh PlatformRefresher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.platformRefresher = refresh
}

func (s *Store) GetPlatform(target PlatformTarget) (PlatformCredential, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.platformCreds[target.Key()]
	return c, ok
}

func (s *Store) SetPlatform(ctx context.Context, target PlatformTarget, credential PlatformCredential) error {
	normalized, err := NewPlatformTarget(target.Platform, target.Origin, target.APIBase)
	if err != nil {
		return err
	}
	target = normalized
	if credential.Type != PlatformPAT && credential.Type != PlatformOAuth {
		return errors.New("invalid platform credential type")
	}
	if strings.TrimSpace(credential.Token) == "" {
		return errors.New("platform credential token is empty")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clonePlatformCredentials(s.platformCreds)
	next[target.Key()] = credential
	if err := s.save(ctx, s.creds, next); err != nil {
		return err
	}
	s.platformCreds = next
	return nil
}

func (s *Store) DeletePlatform(ctx context.Context, target PlatformTarget) error {
	normalized, err := NewPlatformTarget(target.Platform, target.Origin, target.APIBase)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := clonePlatformCredentials(s.platformCreds)
	delete(next, normalized.Key())
	if err := s.save(ctx, s.creds, next); err != nil {
		return err
	}
	s.platformCreds = next
	return nil
}

func PlatformCredentialSource(target PlatformTarget, store *Store) func(context.Context) (PlatformCredential, error) {
	return func(ctx context.Context) (PlatformCredential, error) {
		return ResolvePlatformCredential(ctx, target, store)
	}
}

func ResolvePlatformCredential(ctx context.Context, target PlatformTarget, store *Store) (PlatformCredential, error) {
	normalized, err := NewPlatformTarget(target.Platform, target.Origin, target.APIBase)
	if err != nil {
		return PlatformCredential{}, err
	}
	if normalized.IsPublicCloud() {
		if token := osPlatformToken(normalized.Platform); token != "" {
			return PlatformCredential{Type: PlatformPAT, Token: token}, nil
		}
	}
	if store == nil {
		return PlatformCredential{}, ErrPlatformLoginRequired
	}
	cred, ok := store.GetPlatform(normalized)
	if !ok || cred.Token == "" {
		return PlatformCredential{}, ErrPlatformLoginRequired
	}
	if cred.Type == PlatformPAT || cred.ExpiresAt.IsZero() || time.Until(cred.ExpiresAt) > refreshSkew {
		return cred, nil
	}
	return store.refreshPlatform(ctx, normalized, cred)
}

func (s *Store) refreshPlatform(ctx context.Context, target PlatformTarget, cred PlatformCredential) (PlatformCredential, error) {
	key := target.Key()
	s.refreshMu.Lock()
	if call := s.refreshes[key]; call != nil {
		s.refreshMu.Unlock()
		select {
		case <-call.done:
			return call.cred, call.err
		case <-ctx.Done():
			return PlatformCredential{}, ctx.Err()
		}
	}
	call := &platformRefreshCall{done: make(chan struct{})}
	s.refreshes[key] = call
	s.mu.Lock()
	refresh := s.platformRefresher
	s.mu.Unlock()
	s.refreshMu.Unlock()

	if refresh == nil {
		call.err = ErrPlatformLoginRequired
	} else if next, err := refresh(ctx, target, cred); err != nil {
		call.err = ErrPlatformLoginRequired
	} else if next.Type != PlatformOAuth || next.Token == "" {
		call.err = ErrPlatformLoginRequired
	} else if err := s.SetPlatform(ctx, target, next); err != nil {
		call.err = ErrPlatformLoginRequired
	} else {
		call.cred = next
	}
	s.refreshMu.Lock()
	delete(s.refreshes, key)
	close(call.done)
	s.refreshMu.Unlock()
	return call.cred, call.err
}

type platformRefreshCall struct {
	done chan struct{}
	cred PlatformCredential
	err  error
}

func ApplyPlatformAuth(headers http.Header, platform string, credential PlatformCredential) error {
	headers.Del("PRIVATE-TOKEN")
	headers.Del("Authorization")
	if credential.Token == "" {
		return ErrPlatformLoginRequired
	}
	switch platform {
	case "gitlab":
		if credential.Type == PlatformPAT {
			headers.Set("PRIVATE-TOKEN", credential.Token)
			return nil
		}
		if credential.Type == PlatformOAuth {
			headers.Set("Authorization", "Bearer "+credential.Token)
			return nil
		}
	case "github":
		if credential.Type == PlatformPAT || credential.Type == PlatformOAuth {
			headers.Set("Authorization", "Bearer "+credential.Token)
			return nil
		}
	}
	return errors.New("invalid platform credential")
}
