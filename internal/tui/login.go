package tui

import (
	"context"
	"fmt"

	"github.com/jonathanung/mr-reviewer/internal/auth"
)

// PersistLogin writes credentials to the store. OAuth/device require full tokens
// so OpenAI can run the access_token → API-key exchange. A missing token is an
// error — never report success without persisting.
func PersistLogin(ctx context.Context, store *auth.Store, provider, method string, tokens *auth.Tokens, apiKey string) (string, error) {
	if store == nil {
		return "", fmt.Errorf("no auth store")
	}
	switch method {
	case "key":
		if apiKey == "" {
			return "", fmt.Errorf("empty key")
		}
		if provider == "gitlab" || provider == "github" {
			target, _ := auth.PublicTarget(provider)
			if err := store.SetPlatform(ctx, target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: apiKey}); err != nil {
				return "", err
			}
			return "Stored " + provider + " personal access token", nil
		}
		if err := store.Set(provider, auth.Credential{Type: auth.TypeAPIKey, APIKey: apiKey}); err != nil {
			return "", err
		}
		return "Stored " + provider + " API key", nil
	case "oauth", "device":
		if tokens == nil || tokens.Access == "" {
			return "", fmt.Errorf("missing oauth tokens for %s", provider)
		}
		if provider == "github" {
			return "", fmt.Errorf("GitHub OAuth credentials require a client ID")
		}
		return auth.CompleteLogin(ctx, store, provider, tokens)
	default:
		return "", fmt.Errorf("unknown login method %q", method)
	}
}

// FinishOAuth waits for the pending flow (loopback callback or paste) and
// persists the full token set via CompleteLogin.
func FinishOAuth(ctx context.Context, store *auth.Store, provider string, pending *auth.PendingLogin) (string, error) {
	if pending == nil {
		return "", fmt.Errorf("no pending login")
	}
	tokens, err := pending.Wait(ctx)
	if err != nil {
		return "", err
	}
	if provider == "gitlab" {
		target, _ := auth.PublicTarget(provider)
		return auth.CompletePlatformLogin(ctx, store, target, pending.ClientID(), tokens)
	}
	return PersistLogin(ctx, store, provider, "oauth", tokens, "")
}

// RunDeviceLogin is the shipped xAI device path: request, poll, CompleteLogin.
func RunDeviceLogin(ctx context.Context, store *auth.Store, cfg auth.DeviceConfig) (string, error) {
	if cfg.DeviceURL == "" {
		cfg = auth.XAIDeviceFlow()
	}
	code, err := cfg.RequestCode(ctx)
	if err != nil {
		return "", err
	}
	tokens, err := cfg.Poll(ctx, code)
	if err != nil {
		return "", err
	}
	return PersistLogin(ctx, store, "xai", "device", tokens, "")
}

func devicePrompt(code *auth.DeviceCode) string {
	if code == nil {
		return ""
	}
	uri := code.VerificationURIComplete
	if uri == "" {
		uri = code.VerificationURI
	}
	return fmt.Sprintf("Visit %s and enter %s", uri, code.UserCode)
}
