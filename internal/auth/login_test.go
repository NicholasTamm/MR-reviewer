package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func rewriteDefaultClientHost(t *testing.T, baseURL, host string) {
	t.Helper()
	u, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	old := http.DefaultClient
	transport := http.DefaultTransport
	if old != nil && old.Transport != nil {
		transport = old.Transport
	}
	http.DefaultClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			r2 := req.Clone(req.Context())
			if r2.URL.Host == host {
				r2.URL.Scheme = u.Scheme
				r2.URL.Host = u.Host
				r2.Host = u.Host
			}
			return transport.RoundTrip(r2)
		}),
	}
	t.Cleanup(func() { http.DefaultClient = old })
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeJWT(claims map[string]any) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(claims)
	body := base64.RawURLEncoding.EncodeToString(payload)
	return header + "." + body + ".sig"
}

func TestCompleteLoginXAI(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	msg, err := CompleteLogin(context.Background(), st, "xai", &Tokens{Access: "a", Refresh: "r", IDToken: "id", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || !strings.Contains(msg, "xai") {
		t.Fatalf("msg=%q err=%v", msg, err)
	}
	cred, ok := st.Get("xai")
	if !ok || cred.Access != "a" {
		t.Fatalf("%+v", cred)
	}
}

func TestCompleteLoginOpenAIAccountAndExchange(t *testing.T) {
	var sawExchange bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "urn:ietf:params:oauth:grant-type:token-exchange" {
			t.Errorf("grant = %q", vals.Get("grant_type"))
		}
		if vals.Get("requested_token") != "openai-api-key" {
			t.Errorf("requested_token = %q", vals.Get("requested_token"))
		}
		if vals.Get("subject_token") != "oa" {
			t.Errorf("subject_token = %q", vals.Get("subject_token"))
		}
		if vals.Get("subject_token_type") != "urn:ietf:params:oauth:token-type:access_token" {
			t.Errorf("subject_token_type = %q", vals.Get("subject_token_type"))
		}
		sawExchange = true
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "sk-exchanged", "expires_in": 3600})
	}))
	defer srv.Close()
	rewriteDefaultClientHost(t, srv.URL, "auth.openai.com")
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	idTok := fakeJWT(map[string]any{"chatgpt_account_id": "acct-99"})
	msg, err := CompleteLogin(context.Background(), st, "openai", &Tokens{Access: "oa", Refresh: "or", IDToken: idTok, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if !sawExchange {
		t.Fatal("expected exchange")
	}
	if !strings.Contains(msg, "ChatGPT account detected") || !strings.Contains(msg, "ready for API-backed reviews") {
		t.Errorf("msg = %q", msg)
	}
	cred, ok := st.Get("openai")
	if !ok || cred.AccountID != "acct-99" || cred.APIKey != "sk-exchanged" {
		t.Fatalf("%+v", cred)
	}
}

func TestOpenAIExchangeAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "urn:ietf:params:oauth:grant-type:token-exchange" {
			t.Errorf("grant_type = %q", vals.Get("grant_type"))
		}
		if vals.Get("requested_token") != "openai-api-key" {
			t.Errorf("requested_token = %q", vals.Get("requested_token"))
		}
		if vals.Get("subject_token") != "access-tok" {
			t.Errorf("subject_token = %q", vals.Get("subject_token"))
		}
		if vals.Get("subject_token_type") != "urn:ietf:params:oauth:token-type:access_token" {
			t.Errorf("subject_token_type = %q", vals.Get("subject_token_type"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "sk-new", "expires_in": 1})
	}))
	defer srv.Close()
	rewriteDefaultClientHost(t, srv.URL, "auth.openai.com")
	key, err := OpenAIExchangeAPIKey(context.Background(), "access-tok")
	if err != nil || key != "sk-new" {
		t.Fatalf("key=%q err=%v", key, err)
	}
}
