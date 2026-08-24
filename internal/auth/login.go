package auth

import (
	"context"
	"fmt"
	"os"
)

var exchangeOpenAIAPIKey = OpenAIExchangeAPIKey

func CompleteLogin(ctx context.Context, store *Store, provider string, tokens *Tokens) (string, error) {
	cred := Credential{
		Type:      TypeOAuth,
		Access:    tokens.Access,
		Refresh:   tokens.Refresh,
		IDToken:   tokens.IDToken,
		ExpiresAt: tokens.ExpiresAt,
	}
	message := "Logged in to " + provider
	if provider == "openai" {
		if tokens.IDToken == "" {
			return "", fmt.Errorf("OpenAI login did not return an ID token needed to obtain an API key; use `mr-reviewer auth login openai --api-key` or try signing in again")
		}
		cred.AccountID = AccountIDFromToken(tokens.IDToken)
		if cred.AccountID == "" {
			cred.AccountID = AccountIDFromToken(tokens.Access)
		}
		if cred.AccountID != "" {
			message += " (ChatGPT account detected)"
		}
		key, err := exchangeOpenAIAPIKey(ctx, tokens.IDToken)
		if err != nil {
			return "", fmt.Errorf("OpenAI login could not obtain an API key for API-backed reviews; use `mr-reviewer auth login openai --api-key` or sign in again: %w", err)
		}
		cred.APIKey = key
		message += " (ready for API-backed reviews)"
		if cred.AccountID == "" {
			message += "; warning: no ChatGPT account id in tokens"
		}
	}
	if err := store.Set(provider, cred); err != nil {
		return "", err
	}
	return message, nil
}

func Describe(provider string, store *Store) string {
	provider = canonicalProvider(provider)
	for _, env := range envNames(provider) {
		if os.Getenv(env) != "" {
			return env
		}
	}
	if store == nil {
		return "none"
	}
	if provider == "gitlab" || provider == "github" {
		target, _ := PublicTarget(provider)
		credential, ok := store.GetPlatform(target)
		if !ok {
			return "none"
		}
		if credential.Type == PlatformPAT {
			return "personal access token"
		}
		return "oauth"
	}
	cred, ok := store.Get(provider)
	if !ok {
		return "none"
	}
	switch {
	case cred.Type == TypeAPIKey:
		return "api key"
	case cred.APIKey != "":
		return "oauth+key"
	default:
		return "oauth"
	}
}

func BuiltinProviders() []string {
	return []string{"anthropic", "openai", "xai", "google", "kimi", "deepseek"}
}
