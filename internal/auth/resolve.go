package auth

import (
	"context"
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
	flow, ok := refreshFlows[provider]
	if !ok || cred.Refresh == "" {
		return cred, nil
	}
	tokens, err := flow.Refresh(ctx, cred.Refresh)
	if err != nil {
		return Credential{}, fmt.Errorf("refreshing %s token (run `mr-reviewer auth login %s` if this persists): %w", provider, provider, err)
	}
	cred.Access = tokens.Access
	cred.Refresh = tokens.Refresh
	cred.ExpiresAt = tokens.ExpiresAt
	if tokens.IDToken != "" {
		cred.IDToken = tokens.IDToken
	}
	if err := store.Set(provider, cred); err != nil {
		return Credential{}, fmt.Errorf("persisting refreshed %s token: %w", provider, err)
	}
	return cred, nil
}

func BearerSource(provider string, store *Store) func(ctx context.Context) (string, error) {
	provider = canonicalProvider(provider)
	return func(ctx context.Context) (string, error) {
		if key, ok := APIKey(provider, store); ok {
			return key, nil
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
	if key, ok := APIKey(name, store); ok {
		return key, nil
	}
	return "", fmt.Errorf("no %s token: set %s or run `mr-reviewer auth login %s`", name, envVars[name], name)
}
