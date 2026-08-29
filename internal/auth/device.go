package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type DeviceConfig struct {
	DeviceURL string
	TokenURL  string
	ClientID  string
	Scope     string
	wait      func(context.Context, time.Duration) error
}

type DeviceCode struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

func (d DeviceConfig) RequestCode(ctx context.Context) (*DeviceCode, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.DeviceURL,
		strings.NewReader(url.Values{"client_id": {d.ClientID}, "scope": {d.Scope}}.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code request failed (%s): %.200s", resp.Status, body)
	}
	var code DeviceCode
	if err := json.Unmarshal(body, &code); err != nil {
		return nil, err
	}
	if code.DeviceCode == "" || code.UserCode == "" || code.VerificationURI == "" {
		return nil, fmt.Errorf("device code response missing required fields")
	}
	verificationURL := code.VerificationURIComplete
	if verificationURL == "" {
		verificationURL = code.VerificationURI
	}
	openBrowser(verificationURL)
	return &code, nil
}

func (d DeviceConfig) Poll(ctx context.Context, code *DeviceCode) (*Tokens, error) {
	interval := time.Duration(max(code.Interval, 5)) * time.Second
	expiresIn := code.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 300
	}
	deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
	wait := d.wait
	if wait == nil {
		wait = func(ctx context.Context, interval time.Duration) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(interval):
				return nil
			}
		}
	}

	for time.Now().Before(deadline) {
		if err := wait(ctx, interval); err != nil {
			return nil, err
		}
		resp, err := postForm(ctx, d.TokenURL, url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"client_id":   {d.ClientID},
			"device_code": {code.DeviceCode},
		})
		if err == nil {
			return resp.toTokens(""), nil
		}
		if resp != nil {
			switch resp.Error {
			case "authorization_pending":
				continue
			case "slow_down":
				interval += 5 * time.Second
				continue
			case "access_denied", "authorization_denied":
				return nil, fmt.Errorf("device authorization was denied")
			case "expired_token", "token_expired":
				return nil, fmt.Errorf("device code expired; run login again")
			case "unsupported_grant_type":
				return nil, fmt.Errorf("device authorization failed: unsupported grant type")
			case "incorrect_client_credentials":
				return nil, fmt.Errorf("device authorization failed: invalid OAuth client ID")
			case "incorrect_device_code":
				return nil, fmt.Errorf("device authorization failed: invalid device code")
			case "device_flow_disabled":
				return nil, fmt.Errorf("device authorization is disabled for this OAuth app")
			}
		}
		return nil, err
	}
	return nil, fmt.Errorf("device authorization timed out")
}
