package auth

import (
	"context"
	"errors"
	"os"
	"strings"
)

const (
	gitHubOAuthClientIDEnv = "GITHUB_OAUTH_CLIENT_ID"
	gitHubOAuthClientID    = "Ov23ligm0mrgd7RnRIYU"
)

// GitHubOAuthClientID returns the built-in public OAuth client ID or an override.
func GitHubOAuthClientID() string {
	if clientID := strings.TrimSpace(os.Getenv(gitHubOAuthClientIDEnv)); clientID != "" {
		return clientID
	}
	return gitHubOAuthClientID
}

func GitHubDeviceFlow(clientID string) (DeviceConfig, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return DeviceConfig{}, errors.New("GitHub OAuth requires a client ID: register an OAuth App at https://github.com/settings/developers, enable Device Flow, and pass --client-id or set GITHUB_OAUTH_CLIENT_ID")
	}
	return DeviceConfig{
		DeviceURL: "https://github.com/login/device/code",
		TokenURL:  "https://github.com/login/oauth/access_token",
		ClientID:  clientID,
		Scope:     "repo",
	}, nil
}

// ValidateGitHubScopes verifies the scope returned by GitHub before storage.
func ValidateGitHubScopes(tokens *Tokens) error {
	if tokens == nil {
		return errors.New("GitHub OAuth response is missing tokens")
	}
	for _, scope := range strings.FieldsFunc(tokens.Scope, func(r rune) bool { return r == ',' || r == ' ' }) {
		if scope == "repo" {
			return nil
		}
	}
	return errors.New("GitHub OAuth token is missing the required repo scope; run login again and approve the requested access")
}

func refreshGitHubOAuth(ctx context.Context, credential PlatformCredential) (PlatformCredential, error) {
	flow, err := GitHubDeviceFlow(credential.ClientID)
	if err != nil {
		return PlatformCredential{}, err
	}
	tokens, err := (FlowConfig{TokenURL: flow.TokenURL, ClientID: flow.ClientID}).Refresh(ctx, credential.Refresh)
	if err != nil {
		return PlatformCredential{}, err
	}
	if err := ValidateGitHubScopes(tokens); err != nil {
		return PlatformCredential{}, err
	}
	return PlatformCredential{Type: PlatformOAuth, Token: tokens.Access, Refresh: tokens.Refresh, ExpiresAt: tokens.ExpiresAt, ClientID: credential.ClientID}, nil
}
