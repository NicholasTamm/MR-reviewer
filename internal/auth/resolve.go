package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

var envVars = map[string]string{
	"anthropic": "ANTHROPIC_API_KEY",
	"openai":    "OPENAI_API_KEY",
	"xai":       "XAI_API_KEY",
	"google":    "GEMINI_API_KEY",
	"kimi":      "KIMI_API_KEY",
	"deepseek":  "DEEPSEEK_API_KEY",
	"gitlab":    "GITLAB_TOKEN",
	"github":    "GITHUB_TOKEN",
}

var envAliases = map[string][]string{
	"google": {"GOOGLE_API_KEY"},
}

var refreshFlows = map[string]FlowConfig{
	"openai": OpenAIFlow(),
	"xai":    XAIFlow(),
}

const refreshSkew = 2 * time.Minute

func APIKey(provider string, store *Store) (string, bool) {
	provider = canonicalProvider(provider)
	for _, env := range envNames(provider) {
		if key := os.Getenv(env); key != "" {
			return key, true
		}
	}
	if store != nil {
		if cred, ok := store.Get(provider); ok && cred.APIKey != "" {
			return cred.APIKey, true
		}
	}
	return "", false
}

func envNames(provider string) []string {
	provider = canonicalProvider(provider)
	primary := envVars[provider]
	aliases := envAliases[provider]
	if primary == "" {
		return append([]string(nil), aliases...)
	}
	out := make([]string, 0, 1+len(aliases))
	out = append(out, primary)
	out = append(out, aliases...)
	return out
}

func EnvNames(provider string) []string { return envNames(provider) }

// CredentialsAvailable reports whether a provider has a locally usable credential.
// Network validation is intentionally left to the client onboarding flow.
func CredentialsAvailable(provider string, store *Store, extraEnv string) bool {
	_, ok := CredentialFingerprint(provider, store, extraEnv)
	return ok
}

// CredentialFingerprint returns a stable, secret-free identifier for the
// credential currently selected by normal provider resolution.
func CredentialFingerprint(provider string, store *Store, extraEnv string) (string, bool) {
	provider = canonicalProvider(provider)
	if extraEnv != "" && os.Getenv(extraEnv) != "" {
		return fingerprint(os.Getenv(extraEnv)), true
	}
	if key, ok := APIKey(provider, store); ok {
		return fingerprint(key), true
	}
	if store == nil {
		return "", false
	}
	if provider == "openai" {
		return "", false
	}
	credential, ok := store.Get(provider)
	if !ok || credential.Type != TypeOAuth || credential.Access == "" ||
		(!credential.ExpiresAt.IsZero() && time.Until(credential.ExpiresAt) <= 0) {
		return "", false
	}
	return fingerprint(credential.Access), true
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func freshOAuth(ctx context.Context, store *Store, provider string) (Credential, error) {
	provider = canonicalProvider(provider)
	if store == nil {
		return Credential{}, fmt.Errorf("no OAuth credentials for %s", provider)
	}
	cred, ok := store.Get(provider)
	if !ok || cred.Type != TypeOAuth || cred.Access == "" {
		return Credential{}, fmt.Errorf("no OAuth credentials for %s — run `mr-reviewer auth login %s`", provider, provider)
	}
	if time.Until(cred.ExpiresAt) > refreshSkew {
		return cred, nil
	}
	return store.refreshOAuth(ctx, provider)
}

func (s *Store) refreshOAuth(ctx context.Context, provider string) (Credential, error) {
	s.oauthRefreshMu.Lock()
	if call := s.oauthRefreshes[provider]; call != nil {
		s.oauthRefreshMu.Unlock()
		select {
		case <-call.done:
			return call.cred, call.err
		case <-ctx.Done():
			return Credential{}, ctx.Err()
		}
	}
	call := &oauthRefreshCall{done: make(chan struct{})}
	s.oauthRefreshes[provider] = call
	s.oauthRefreshMu.Unlock()

	// Re-read after joining the refresh flight so followers use the persisted rotation.
	cred, ok := s.Get(provider)
	if !ok || cred.Type != TypeOAuth || cred.Access == "" {
		call.err = fmt.Errorf("no OAuth credentials for %s — run `mr-reviewer auth login %s`", provider, provider)
	} else if time.Until(cred.ExpiresAt) > refreshSkew {
		call.cred = cred
	} else if flow, ok := refreshFlows[provider]; !ok || cred.Refresh == "" {
		call.cred = cred
	} else if tokens, err := flow.Refresh(ctx, cred.Refresh); err != nil {
		call.err = fmt.Errorf("refreshing %s token (run `mr-reviewer auth login %s` if this persists): %w", provider, provider, err)
	} else {
		cred.Access = tokens.Access
		cred.Refresh = tokens.Refresh
		cred.ExpiresAt = tokens.ExpiresAt
		if tokens.IDToken != "" {
			cred.IDToken = tokens.IDToken
		}
		if err := s.Set(provider, cred); err != nil {
			call.err = fmt.Errorf("persisting refreshed %s token: %w", provider, err)
		} else {
			call.cred = cred
		}
	}

	s.oauthRefreshMu.Lock()
	delete(s.oauthRefreshes, provider)
	close(call.done)
	s.oauthRefreshMu.Unlock()
	return call.cred, call.err
}

type oauthRefreshCall struct {
	done chan struct{}
	cred Credential
	err  error
}

func openAIAPIKey(ctx context.Context, store *Store) (string, error) {
	cred, err := freshOAuth(ctx, store, "openai")
	if err != nil {
		return "", fmt.Errorf("OpenAI requires a standard API key: set OPENAI_API_KEY or run `mr-reviewer auth login openai` again")
	}
	key, err := exchangeOpenAIAPIKey(ctx, cred.Access)
	if err != nil {
		return "", fmt.Errorf("OpenAI OAuth credentials could not obtain a standard API key; run `mr-reviewer auth login openai` again: %w", err)
	}
	cred.APIKey = key
	if err := store.Set("openai", cred); err != nil {
		return "", fmt.Errorf("persisting exchanged OpenAI API key: %w", err)
	}
	return key, nil
}

func BearerSourceEnv(provider string, store *Store, envName string) func(ctx context.Context) (string, error) {
	provider = canonicalProvider(provider)
	return func(ctx context.Context) (string, error) {
		if envName != "" {
			if key := os.Getenv(envName); key != "" {
				return key, nil
			}
		}
		if key, ok := APIKey(provider, store); ok {
			return key, nil
		}
		if provider == "openai" {
			return openAIAPIKey(ctx, store)
		}
		cred, err := freshOAuth(ctx, store, provider)
		if err != nil {
			hint := envName
			if hint == "" {
				hint = "an API key"
			}
			return "", fmt.Errorf("no credentials for %s: set %s or run `mr-reviewer auth login %s`", provider, hint, provider)
		}
		return cred.Access, nil
	}
}

func BearerSource(provider string, store *Store) func(ctx context.Context) (string, error) {
	provider = canonicalProvider(provider)
	return func(ctx context.Context) (string, error) {
		if key, ok := APIKey(provider, store); ok {
			return key, nil
		}
		if provider == "openai" {
			return openAIAPIKey(ctx, store)
		}
		cred, err := freshOAuth(ctx, store, provider)
		if err != nil {
			env := envVars[provider]
			if env == "" {
				env = "an API key"
			}
			return "", fmt.Errorf("no credentials for %s: set %s or run `mr-reviewer auth login %s`", provider, env, provider)
		}
		return cred.Access, nil
	}
}

func PlatformToken(name string, store *Store) (string, error) {
	target, err := PublicTarget(name)
	if err != nil {
		return "", err
	}
	credential, err := ResolvePlatformCredential(context.Background(), target, store)
	if err != nil {
		return "", fmt.Errorf("no %s token: set %s or run `mr-reviewer auth login %s`", name, envVars[name], name)
	}
	return credential.Token, nil
}

func osPlatformToken(platform string) string {
	return os.Getenv(envVars[platform])
}
