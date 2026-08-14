package auth

import (
	"context"
	"fmt"
	"net/url"
)

const (
	openaiIssuer       = "https://auth.openai.com"
	openaiClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	openaiRedirectPort = 1455
)

func OpenAIFlow() FlowConfig {
	return FlowConfig{
		AuthorizeURL: openaiIssuer + "/oauth/authorize",
		TokenURL:     openaiIssuer + "/oauth/token",
		ClientID:     openaiClientID,
		Scope:        "openid profile email offline_access",
		RedirectHost: "localhost",
		RedirectPort: openaiRedirectPort,
		RedirectPath: "/auth/callback",
		ExtraParams: map[string]string{
			"id_token_add_organizations": "true",
			"codex_cli_simplified_flow":  "true",
			"originator":                 "codex_cli_rs",
		},
	}
}

// OpenAIExchangeAPIKey trades the OAuth id_token for a standard OpenAI API key.
func OpenAIExchangeAPIKey(ctx context.Context, idToken string) (string, error) {
	resp, err := postForm(ctx, openaiIssuer+"/oauth/token", url.Values{
		"grant_type":         {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"client_id":          {openaiClientID},
		"requested_token":    {"openai-api-key"},
		"subject_token":      {idToken},
		"subject_token_type": {"urn:ietf:params:oauth:token-type:id_token"},
	})
	if err != nil {
		return "", fmt.Errorf("API key exchange: %w", err)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("API key exchange returned an empty key")
	}
	return resp.AccessToken, nil
}
