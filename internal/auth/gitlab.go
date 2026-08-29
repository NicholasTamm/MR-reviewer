package auth

import (
	"context"
	"errors"
	"os"
	"strings"
)

const (
	gitLabOAuthClientIDEnv = "GITLAB_OAUTH_CLIENT_ID"
	gitLabOAuthClientID    = "299626a83068c088b7cc99be9411c82afce5cfc89b3a9810e6167c3944457aa9"
)

// GitLabOAuthClientID returns the built-in public OAuth client ID or an override.
func GitLabOAuthClientID() string {
	if clientID := strings.TrimSpace(os.Getenv(gitLabOAuthClientIDEnv)); clientID != "" {
		return clientID
	}
	return gitLabOAuthClientID
}

func GitLabFlow(clientID string) (FlowConfig, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return FlowConfig{}, errors.New("GitLab OAuth requires a client ID: register an application at https://gitlab.com/-/user_settings/applications with redirect URI http://127.0.0.1:8620/oauth/callback and pass --client-id or set GITLAB_OAUTH_CLIENT_ID")
	}
	return FlowConfig{
		AuthorizeURL: "https://gitlab.com/oauth/authorize",
		TokenURL:     "https://gitlab.com/oauth/token",
		ClientID:     clientID,
		Scope:        "api",
		RedirectHost: "127.0.0.1",
		RedirectPort: 8620,
		RedirectPath: "/oauth/callback",
	}, nil
}

func GitLabDeviceFlow(clientID string) (DeviceConfig, error) {
	if _, err := GitLabFlow(clientID); err != nil {
		return DeviceConfig{}, err
	}
	return DeviceConfig{
		DeviceURL: "https://gitlab.com/oauth/authorize_device",
		TokenURL:  "https://gitlab.com/oauth/token",
		ClientID:  strings.TrimSpace(clientID),
		Scope:     "api",
	}, nil
}

func refreshPlatformOAuth(ctx context.Context, target PlatformTarget, credential PlatformCredential) (PlatformCredential, error) {
	if !target.IsPublicCloud() || credential.Type != PlatformOAuth || credential.Refresh == "" || credential.ClientID == "" {
		return PlatformCredential{}, ErrPlatformLoginRequired
	}
	switch target.Platform {
	case "gitlab":
		flow, err := GitLabFlow(credential.ClientID)
		if err != nil {
			return PlatformCredential{}, err
		}
		tokens, err := flow.Refresh(ctx, credential.Refresh)
		if err != nil {
			return PlatformCredential{}, err
		}
		return PlatformCredential{Type: PlatformOAuth, Token: tokens.Access, Refresh: tokens.Refresh, ExpiresAt: tokens.ExpiresAt, ClientID: credential.ClientID}, nil
	case "github":
		return refreshGitHubOAuth(ctx, credential)
	default:
		return PlatformCredential{}, ErrPlatformLoginRequired
	}
}

func CompletePlatformLogin(ctx context.Context, store *Store, target PlatformTarget, clientID string, tokens *Tokens) (string, error) {
	if store == nil || tokens == nil || tokens.Access == "" {
		return "", errors.New("missing platform OAuth tokens")
	}
	clientID = strings.TrimSpace(clientID)
	if target.Platform == "github" {
		if !target.IsPublicCloud() {
			return "", errors.New("GitHub OAuth device login supports GitHub.com only")
		}
		if clientID == "" {
			return "", errors.New("GitHub OAuth credentials require a client ID")
		}
		if err := ValidateGitHubScopes(tokens); err != nil {
			return "", err
		}
	}
	if err := store.SetPlatform(ctx, target, PlatformCredential{Type: PlatformOAuth, Token: tokens.Access, Refresh: tokens.Refresh, ExpiresAt: tokens.ExpiresAt, ClientID: clientID}); err != nil {
		return "", err
	}
	return "Logged in to " + target.Platform, nil
}
