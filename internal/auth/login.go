package auth

import (
	"context"
	"os"
)

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
		cred.AccountID = AccountIDFromToken(tokens.IDToken)
		if cred.AccountID == "" {
			cred.AccountID = AccountIDFromToken(tokens.Access)
		}
		if cred.AccountID != "" {
			message += " (ChatGPT subscription mode)"
		}
		if tokens.IDToken != "" {
			if key, err := OpenAIExchangeAPIKey(ctx, tokens.IDToken); err == nil {
				cred.APIKey = key
			}
		}
		if cred.AccountID == "" {
			message += " — warning: no ChatGPT account id in tokens; subscription mode unavailable"
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
