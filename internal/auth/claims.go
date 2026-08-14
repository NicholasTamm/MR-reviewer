package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

type idTokenClaims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Auth             struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"https://api.openai.com/auth"`
	Organizations []struct {
		ID string `json:"id"`
	} `json:"organizations"`
}

// AccountIDFromToken extracts the ChatGPT account id from a JWT without verifying it.
func AccountIDFromToken(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var claims idTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	switch {
	case claims.ChatGPTAccountID != "":
		return claims.ChatGPTAccountID
	case claims.Auth.ChatGPTAccountID != "":
		return claims.Auth.ChatGPTAccountID
	case len(claims.Organizations) > 0:
		return claims.Organizations[0].ID
	}
	return ""
}
