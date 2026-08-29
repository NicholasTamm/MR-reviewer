package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestXAIDeviceFlowShape(t *testing.T) {
	d := XAIDeviceFlow()
	if d.ClientID != xaiClientID || !strings.Contains(d.DeviceURL, "/oauth2/device/code") || !strings.Contains(d.TokenURL, "/oauth2/token") || d.Scope == "" {
		t.Fatalf("%+v", d)
	}
}

func TestDeviceRequestCodeSuccess(t *testing.T) {
	opened := stubOpenBrowser(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("client_id") != "cid" || vals.Get("scope") != "s" {
			t.Errorf("form = %v", vals)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dev-code", "user_code": "USER-1",
			"verification_uri": "https://verify.test/", "expires_in": 600, "interval": 5,
		})
	}))
	defer srv.Close()
	code, err := DeviceConfig{DeviceURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "cid", Scope: "s"}.RequestCode(context.Background())
	if err != nil || code.DeviceCode != "dev-code" || code.UserCode != "USER-1" {
		t.Fatalf("%+v %v", code, err)
	}
	if len(*opened) != 1 || (*opened)[0] != "https://verify.test/" {
		t.Fatalf("opened = %v", *opened)
	}
}

func TestDeviceRequestCodeErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()
	_, err := DeviceConfig{DeviceURL: srv.URL, ClientID: "c"}.RequestCode(context.Background())
	if err == nil || !strings.Contains(err.Error(), "device code request failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestDevicePollSuccess(t *testing.T) {
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" {
			t.Errorf("grant = %q", vals.Get("grant_type"))
		}
		if n.Add(1) == 1 {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-dev", "refresh_token": "refresh-dev", "expires_in": 120})
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	tok, err := DeviceConfig{TokenURL: srv.URL, ClientID: "cid"}.Poll(ctx, &DeviceCode{DeviceCode: "dc", Interval: 1, ExpiresIn: 60})
	if err != nil || tok.Access != "access-dev" {
		t.Fatalf("%+v %v", tok, err)
	}
}

func TestDevicePollDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "access_denied"})
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := DeviceConfig{TokenURL: srv.URL, ClientID: "c"}.Poll(ctx, &DeviceCode{DeviceCode: "dc", Interval: 5, ExpiresIn: 30})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "denied") {
		t.Fatalf("err = %v", err)
	}
}

func TestDevicePollContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
	}))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, err := DeviceConfig{TokenURL: srv.URL, ClientID: "c"}.Poll(ctx, &DeviceCode{DeviceCode: "dc", Interval: 5, ExpiresIn: 300})
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("err = %v", err)
	}
}

func TestDevicePollBacksOffAndHandlesDocumentedTerminalErrors(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch requests.Add(1) {
		case 1:
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
		case 2:
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "slow_down"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "scope": "repo"})
		}
	}))
	defer srv.Close()
	var waits []time.Duration
	flow := DeviceConfig{TokenURL: srv.URL, ClientID: "cid", wait: func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}}
	tokens, err := flow.Poll(context.Background(), &DeviceCode{DeviceCode: "dc", Interval: 5, ExpiresIn: 60})
	if err != nil || tokens.Access != "access" || tokens.Scope != "repo" {
		t.Fatalf("tokens=%+v err=%v", tokens, err)
	}
	if len(waits) != 3 || waits[0] != 5*time.Second || waits[1] != 5*time.Second || waits[2] != 10*time.Second {
		t.Fatalf("waits = %v", waits)
	}

	for code, want := range map[string]string{
		"expired_token":                "expired",
		"unsupported_grant_type":       "unsupported grant type",
		"incorrect_client_credentials": "invalid OAuth client ID",
		"incorrect_device_code":        "invalid device code",
		"device_flow_disabled":         "disabled",
	} {
		t.Run(code, func(t *testing.T) {
			terminal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
			}))
			defer terminal.Close()
			_, err := (DeviceConfig{TokenURL: terminal.URL, ClientID: "cid", wait: func(context.Context, time.Duration) error { return nil }}).Poll(context.Background(), &DeviceCode{DeviceCode: "dc", ExpiresIn: 60})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
