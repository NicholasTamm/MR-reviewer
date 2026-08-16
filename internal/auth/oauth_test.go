package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func stubOpenBrowser(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	prev := openBrowser
	openBrowser = func(target string) { opened = append(opened, target) }
	t.Cleanup(func() { openBrowser = prev })
	return &opened
}

func testOAuthFlow(tokenURL string, port int) FlowConfig {
	return FlowConfig{
		AuthorizeURL: "http://issuer.test/oauth/authorize",
		TokenURL:     tokenURL,
		ClientID:     "test-client",
		Scope:        "openid",
		RedirectHost: "127.0.0.1",
		RedirectPort: port,
		RedirectPath: "/auth/callback",
	}
}

func beginTestPending(t *testing.T, tokenURL string) *PendingLogin {
	t.Helper()
	stubOpenBrowser(t)
	p, err := testOAuthFlow(tokenURL, freeLoopbackPort(t)).Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p.server != nil {
			_ = p.server.Close()
		}
	})
	return p
}

func pendingState(t *testing.T, p *PendingLogin) string {
	t.Helper()
	u, err := url.Parse(p.URL)
	if err != nil {
		t.Fatal(err)
	}
	state := u.Query().Get("state")
	if state == "" {
		t.Fatal("missing state")
	}
	return state
}

func tokenExchangeServer(t *testing.T, access string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "authorization_code" || vals.Get("code") == "" || vals.Get("code_verifier") == "" {
			t.Errorf("form = %v", vals)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": access, "refresh_token": "refresh-test", "expires_in": 3600,
		})
	}))
}

func TestCompleteWithPasteFullCallbackURLExchanges(t *testing.T) {
	srv := tokenExchangeServer(t, "access-from-paste")
	defer srv.Close()
	p := beginTestPending(t, srv.URL)
	state := pendingState(t, p)
	callback := fmt.Sprintf("http://127.0.0.1:%d/auth/callback?code=%s&state=%s", p.flow.RedirectPort, "auth-code", url.QueryEscape(state))
	if err := p.CompleteWithPaste(callback); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tok, err := p.Wait(ctx)
	if err != nil || tok.Access != "access-from-paste" {
		t.Fatalf("tok=%+v err=%v", tok, err)
	}
}

func TestCompleteWithPasteWrongStateAllowsRetry(t *testing.T) {
	p := beginTestPending(t, "http://127.0.0.1:1/unused-token")
	state := pendingState(t, p)
	secret := "SECRET_CODE_MUST_NOT_LEAK"
	err := p.CompleteWithPaste("http://127.0.0.1/cb?code=" + secret + "&state=wrong")
	if err == nil || !strings.Contains(err.Error(), "state mismatch") || strings.Contains(err.Error(), secret) {
		t.Fatalf("err = %v", err)
	}
	if err := p.CompleteWithPaste("http://127.0.0.1/cb?code=good&state=" + url.QueryEscape(state)); err != nil {
		t.Fatal(err)
	}
}

func TestCompleteWithPasteBareCode(t *testing.T) {
	p := beginTestPending(t, "http://127.0.0.1:1/unused")
	if err := p.CompleteWithPaste("  bare-authorization-code-abc  "); err != nil {
		t.Fatal(err)
	}
}

func TestBeginReturnsPendingWhenBindFails(t *testing.T) {
	stubOpenBrowser(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	p, err := testOAuthFlow("http://127.0.0.1:1/unused", port).Begin()
	if err != nil {
		t.Fatal(err)
	}
	if p.LoopbackListening() {
		t.Error("expected bind failure")
		_ = p.server.Close()
	}
	if err := p.CompleteWithPaste("paste-without-listener"); err != nil {
		t.Fatal(err)
	}
}

func TestLoginErrorsImmediatelyWhenBindFails(t *testing.T) {
	stubOpenBrowser(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = testOAuthFlow("http://127.0.0.1:1/unused", port).Login(ctx)
	if err == nil || !strings.Contains(err.Error(), "cannot bind") || !strings.Contains(err.Error(), "TUI") {
		t.Fatalf("err = %v", err)
	}
}

func TestHTTPCallbackStillWorksWhenBindSucceeds(t *testing.T) {
	srv := tokenExchangeServer(t, "access-from-http")
	defer srv.Close()
	p := beginTestPending(t, srv.URL)
	if p.server == nil {
		t.Fatal("expected loopback server")
	}
	state := pendingState(t, p)
	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/auth/callback?code=http-code&state=%s", p.flow.RedirectPort, url.QueryEscape(state))
	errCh := make(chan error, 1)
	tokCh := make(chan *Tokens, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		tok, err := p.Wait(ctx)
		if err != nil {
			errCh <- err
			return
		}
		tokCh <- tok
	}()
	time.Sleep(20 * time.Millisecond)
	resp, err := http.Get(callbackURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "Login complete") {
		t.Errorf("page = %q", body)
	}
	select {
	case err := <-errCh:
		t.Fatal(err)
	case tok := <-tokCh:
		if tok.Access != "access-from-http" {
			t.Errorf("access = %q", tok.Access)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout")
	}
}

func TestParseOAuthPasteTable(t *testing.T) {
	cases := []struct {
		raw, wantCode, wantState, wantErr string
	}{
		{"", "", "", "empty"},
		{"  bare  ", "bare", "", ""},
		{"http://h/cb?code=c1&state=s1", "c1", "s1", ""},
		{"code=c2&state=s2", "c2", "s2", ""},
		{"http://h/cb?error=denied&error_description=nope", "", "", "nope"},
	}
	for _, tc := range cases {
		code, state, err := parseOAuthPaste(tc.raw)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("parse(%q) err=%v", tc.raw, err)
			}
			continue
		}
		if err != nil || code != tc.wantCode || state != tc.wantState {
			t.Errorf("parse(%q)=(%q,%q,%v)", tc.raw, code, state, err)
		}
	}
}

func TestFlowRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("grant_type") != "refresh_token" {
			t.Errorf("grant = %q", vals.Get("grant_type"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access", "refresh_token": "new-refresh", "expires_in": 1800,
		})
	}))
	defer srv.Close()
	tok, err := FlowConfig{TokenURL: srv.URL, ClientID: "cid"}.Refresh(context.Background(), "old-refresh")
	if err != nil || tok.Access != "new-access" || tok.Refresh != "new-refresh" {
		t.Fatalf("%+v %v", tok, err)
	}
}
