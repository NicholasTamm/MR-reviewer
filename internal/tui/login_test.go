package tui

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

func tokenServer(t *testing.T, access string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", vals.Get("grant_type"))
		}
		if vals.Get("code") == "" || vals.Get("code_verifier") == "" {
			t.Errorf("missing code/verifier: %v", vals)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": access, "refresh_token": "refresh-tui", "id_token": "id-tui", "expires_in": 3600,
		})
	}))
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func testFlow(tokenURL string, port int) auth.FlowConfig {
	return auth.FlowConfig{
		AuthorizeURL: "http://issuer.test/oauth/authorize",
		TokenURL:     tokenURL,
		ClientID:     "test-client",
		Scope:        "openid",
		RedirectHost: "127.0.0.1",
		RedirectPort: port,
		RedirectPath: "/auth/callback",
	}
}

func TestPersistLoginOAuthWritesStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := PersistLogin(context.Background(), st, "xai", "oauth", &auth.Tokens{
		Access: "acc-full", Refresh: "ref-full", IDToken: "id-full", ExpiresAt: time.Now().Add(time.Hour),
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "xai") {
		t.Fatalf("msg = %q", msg)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o", info.Mode().Perm())
	}
	got, ok := st.Get("xai")
	if !ok || got.Access != "acc-full" || got.Refresh != "ref-full" || got.Type != auth.TypeOAuth {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestPersistLoginOAuthWithoutTokensErrors(t *testing.T) {
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PersistLogin(context.Background(), st, "openai", "oauth", nil, "not-a-token"); err == nil {
		t.Fatal("expected error without tokens")
	}
	if _, ok := st.Get("openai"); ok {
		t.Fatal("must not persist a fake login")
	}
}

func TestFinishOAuthAfterPastePersists(t *testing.T) {
	srv := tokenServer(t, "access-from-finish")
	defer srv.Close()
	p, err := testFlow(srv.URL, freePort(t)).Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p != nil && p.LoopbackListening() {
			// Wait closes the server
		}
	})
	if err := p.CompleteWithPaste("paste-code-xyz"); err != nil {
		t.Fatal(err)
	}
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	msg, err := FinishOAuth(ctx, st, "xai", p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "xai") {
		t.Fatalf("msg = %q", msg)
	}
	got, ok := st.Get("xai")
	if !ok || got.Access != "access-from-finish" || got.Refresh != "refresh-tui" {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestRunDeviceLoginPersists(t *testing.T) {
	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "device") || r.URL.Path == "/" && polls == 0 && r.Header.Get("X") == "" {
			// first hit is RequestCode if DeviceURL == TokenURL we need to distinguish
		}
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" {
			polls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "dev-access", "refresh_token": "dev-refresh", "expires_in": 120,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "USER-9",
			"verification_uri": "https://verify.test/", "expires_in": 60, "interval": 5,
		})
	}))
	defer srv.Close()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	msg, err := RunDeviceLogin(ctx, st, auth.DeviceConfig{
		DeviceURL: srv.URL, TokenURL: srv.URL, ClientID: "cid", Scope: "s",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "xai") {
		t.Fatalf("msg = %q", msg)
	}
	got, ok := st.Get("xai")
	if !ok || got.Access != "dev-access" || got.Refresh != "dev-refresh" {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestLiveSessionLoginOAuthDoesNotFakeSuccess(t *testing.T) {
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &liveSession{store: st}
	msg, err := s.login("openai", "oauth", "only-access-string")
	if err == nil {
		t.Fatalf("expected error, got %q", msg)
	}
	if _, ok := st.Get("openai"); ok {
		t.Fatal("stub oauth must not write the store")
	}
}

func TestLiveSessionLoginKeyPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	s := &liveSession{store: st}
	msg, err := s.login("anthropic", "key", "sk-ant-test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "anthropic") {
		t.Fatalf("msg = %q", msg)
	}
	got, ok := st.Get("anthropic")
	if !ok || got.APIKey != "sk-ant-test" {
		t.Fatalf("stored = %+v", got)
	}
}

func TestLiveSessionDeviceLoginPersists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "sess-dev", "refresh_token": "sess-ref", "expires_in": 60,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "ABCD",
			"verification_uri": "https://verify.test/", "expires_in": 60, "interval": 5,
		})
	}))
	defer srv.Close()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &liveSession{
		store:  st,
		device: auth.DeviceConfig{DeviceURL: srv.URL, TokenURL: srv.URL, ClientID: "c", Scope: "s"},
	}
	msg, err := s.login("xai", "device", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "xai") {
		t.Fatalf("msg = %q", msg)
	}
	got, ok := st.Get("xai")
	if !ok || got.Access != "sess-dev" {
		t.Fatalf("stored = %+v ok=%v", got, ok)
	}
}

func TestUpdateOAuthPasteFinishesAndPersists(t *testing.T) {
	srv := tokenServer(t, "access-from-update")
	defer srv.Close()
	// Hold a port so Begin cannot bind — paste path.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	flow := testFlow(srv.URL, port)

	path := filepath.Join(t.TempDir(), "auth.json")
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		Store:    st,
		Settings: config.Settings{Provider: "echo", Model: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadDash: func(string) ([]review.ProjectMergeRequests, error) { return nil, nil },
		BeginOAuth: func(provider string) (*auth.PendingLogin, error) {
			if provider != "xai" {
				t.Fatalf("provider = %s", provider)
			}
			return flow.Begin()
		},
	})
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('a'))
	// xai is index 2
	m, _ = applyKey(m, special(tea.KeyDown))
	m, _ = applyKey(m, special(tea.KeyDown))
	m, cmd := applyKey(m, special(tea.KeyEnter))
	if cmd != nil {
		// loopback failed — no wait cmd until paste
		t.Fatal("bind-fail paste path must not start Wait before paste")
	}
	if m.input != inputOAuthPaste {
		t.Fatalf("input = %d, want paste", m.input)
	}
	for _, r := range "pasted-auth-code" {
		m, _ = applyKey(m, key(r))
	}
	m, cmd = applyKey(m, special(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("paste commit must return Wait+CompleteLogin cmd")
	}
	m = drain(t, m, cmd)
	if m.Status() == "" && m.err == "" {
		// status should mention logged in
	}
	if !strings.Contains(m.Status(), "xai") && !strings.Contains(m.Status(), "Logged") {
		t.Fatalf("status = %q err=%q", m.Status(), m.Error())
	}
	got, ok := st.Get("xai")
	if !ok || got.Access != "access-from-update" || got.Refresh != "refresh-tui" {
		t.Fatalf("store after Update paste = %+v ok=%v", got, ok)
	}
}

func TestUpdateDeviceLoginPersists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "upd-dev", "refresh_token": "upd-ref", "expires_in": 60,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"device_code": "dc", "user_code": "WXYZ",
			"verification_uri": "https://verify.test/go", "expires_in": 60, "interval": 5,
		})
	}))
	defer srv.Close()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		Store:    st,
		Settings: config.Settings{Provider: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadDash: func(string) ([]review.ProjectMergeRequests, error) { return nil, nil },
		Device:   auth.DeviceConfig{DeviceURL: srv.URL, TokenURL: srv.URL, ClientID: "c", Scope: "s"},
	})
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('a'))
	// xai is index 2
	m, _ = applyKey(m, special(tea.KeyDown))
	m, _ = applyKey(m, special(tea.KeyDown))
	if m.authList[m.authCursor] != "xai" {
		t.Fatalf("cursor provider = %s", m.authList[m.authCursor])
	}
	m, cmd := applyKey(m, key('d'))
	if cmd == nil {
		t.Fatal("device login must return a command")
	}
	m, cmd2 := applyUpdate(m, cmd())
	if !strings.Contains(m.Status(), "WXYZ") {
		t.Fatalf("expected user code in status: %q", m.Status())
	}
	if cmd2 == nil {
		t.Fatal("device code msg must start Poll")
	}
	m = drain(t, m, cmd2)
	got, ok := st.Get("xai")
	if !ok || got.Access != "upd-dev" {
		t.Fatalf("store = %+v ok=%v status=%q", got, ok, m.Status())
	}
}

func applyUpdate(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestStartLoginLoopbackPersistsDespiteStubLogin(t *testing.T) {
	srv := tokenServer(t, "access-loopback")
	defer srv.Close()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		Store:    st,
		Settings: config.Settings{Provider: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadDash: func(string) ([]review.ProjectMergeRequests, error) { return nil, nil },
		Login: func(provider, method, secret string) (string, error) {
			t.Fatalf("loginFn must not persist oauth (got %s %s %s)", provider, method, secret)
			return "FAKE", nil
		},
		BeginOAuth: func(provider string) (*auth.PendingLogin, error) {
			return testFlow(srv.URL, freePort(t)).Begin()
		},
	})
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('a'))
	m, _ = applyKey(m, special(tea.KeyDown))
	m, _ = applyKey(m, special(tea.KeyDown))
	m, cmd := applyKey(m, special(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("listening loopback should start Wait+CompleteLogin")
	}
	if m.pending == nil {
		t.Fatal("pending missing")
	}
	if err := m.pending.CompleteWithPaste("loop-code"); err != nil {
		t.Fatal(err)
	}
	m = drain(t, m, cmd)
	got, ok := st.Get("xai")
	if !ok || got.Access != "access-loopback" || got.Refresh != "refresh-tui" {
		t.Fatalf("store = %+v ok=%v status=%q", got, ok, m.Status())
	}
}

func TestConfigureChoosesModel(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('l'))
	m, _ = applyKey(m, special(tea.KeyTab)) // provider
	m, _ = applyKey(m, special(tea.KeyTab)) // model
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.input != inputModel {
		t.Fatalf("enter on model field: input=%d, want inputModel; view=%s", m.input, m.ViewName())
	}
	// replace default by typing
	before := m.ModelID()
	m, _ = applyKey(m, key('g'))
	m, _ = applyKey(m, key('p'))
	m, _ = applyKey(m, key('t'))
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.input != inputNone {
		t.Fatalf("input still %d", m.input)
	}
	if !strings.HasSuffix(m.ModelID(), "gpt") && m.ModelID() == before {
		t.Fatalf("model unchanged: %q", m.ModelID())
	}
	if !strings.Contains(m.ModelID(), "gpt") && !strings.Contains(m.ModelID(), "g") {
		t.Fatalf("model = %q", m.ModelID())
	}
}

func TestConfigureTypeEntersModelField(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('l'))
	m, _ = applyKey(m, special(tea.KeyTab))
	m, _ = applyKey(m, special(tea.KeyTab))
	m, _ = applyKey(m, key('c'))
	if m.input != inputModel {
		t.Fatalf("typing on model: input=%d", m.input)
	}
	if !strings.Contains(m.ModelID(), "c") {
		t.Fatalf("model = %q", m.ModelID())
	}
}
