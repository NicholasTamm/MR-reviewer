package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type FlowConfig struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	Scope        string
	RedirectHost string
	RedirectPort int
	RedirectPath string
	ExtraParams  map[string]string
	IncludeNonce bool
}

type Tokens struct {
	Access    string
	Refresh   string
	IDToken   string
	Scope     string
	ExpiresAt time.Time
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (r tokenResponse) toTokens(fallbackRefresh string) *Tokens {
	refresh := r.RefreshToken
	if refresh == "" {
		refresh = fallbackRefresh
	}
	tokens := &Tokens{
		Access:  r.AccessToken,
		Refresh: refresh,
		IDToken: r.IDToken,
		Scope:   r.Scope,
	}
	if r.ExpiresIn > 0 {
		tokens.ExpiresAt = time.Now().Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	return tokens
}

func (f FlowConfig) redirectURI() string {
	return fmt.Sprintf("http://%s:%d%s", f.RedirectHost, f.RedirectPort, f.RedirectPath)
}

type callbackResult struct {
	code string
	err  error
}

type PendingLogin struct {
	URL     string
	flow    FlowConfig
	pkce    pkceCodes
	state   string
	results chan callbackResult
	server  *http.Server
	once    sync.Once
}

func (f FlowConfig) Begin() (*PendingLogin, error) {
	if strings.TrimSpace(f.AuthorizeURL) == "" || strings.TrimSpace(f.TokenURL) == "" || strings.TrimSpace(f.ClientID) == "" || strings.TrimSpace(f.Scope) == "" || strings.TrimSpace(f.RedirectHost) == "" || f.RedirectPort <= 0 || strings.TrimSpace(f.RedirectPath) == "" {
		return nil, fmt.Errorf("incomplete OAuth configuration")
	}
	pkce := newPKCE()
	state := randomURLSafe(32)

	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {f.ClientID},
		"redirect_uri":          {f.redirectURI()},
		"scope":                 {f.Scope},
		"code_challenge":        {pkce.challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	if f.IncludeNonce {
		params.Set("nonce", randomURLSafe(32))
	}
	for k, v := range f.ExtraParams {
		params.Set(k, v)
	}
	authorizeURL := f.AuthorizeURL + "?" + params.Encode()

	p := &PendingLogin{
		URL:     authorizeURL,
		flow:    f,
		pkce:    pkce,
		state:   state,
		results: make(chan callbackResult, 1),
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", f.RedirectHost, f.RedirectPort))
	if err == nil {
		p.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != f.RedirectPath {
				http.NotFound(w, r)
				return
			}
			q := r.URL.Query()
			switch {
			case q.Get("error") != "":
				msg := q.Get("error_description")
				if msg == "" {
					msg = q.Get("error")
				}
				writeCallbackPage(w, "Login failed: "+msg)
				_ = p.deliver(callbackResult{err: fmt.Errorf("authorization failed: %s", msg)})
			case q.Get("state") != state:
				writeCallbackPage(w, "Login failed: state mismatch.")
				_ = p.deliver(callbackResult{err: fmt.Errorf("state mismatch in OAuth callback (possible CSRF)")})
			case q.Get("code") == "":
				writeCallbackPage(w, "Login failed: missing authorization code.")
				_ = p.deliver(callbackResult{err: fmt.Errorf("callback missing authorization code")})
			default:
				writeCallbackPage(w, "Login complete. You can close this window and return to mr-reviewer.")
				_ = p.deliver(callbackResult{code: q.Get("code")})
			}
		})}
		go p.server.Serve(ln)
	}

	openBrowser(authorizeURL)
	return p, nil
}

func (p *PendingLogin) ClientID() string {
	if p == nil {
		return ""
	}
	return p.flow.ClientID
}

func (p *PendingLogin) LoopbackListening() bool {
	return p != nil && p.server != nil
}

func (p *PendingLogin) deliver(res callbackResult) error {
	delivered := false
	p.once.Do(func() {
		p.results <- res
		delivered = true
	})
	if !delivered {
		return fmt.Errorf("login already completed")
	}
	return nil
}

func (p *PendingLogin) Wait(ctx context.Context) (*Tokens, error) {
	if p.server != nil {
		defer p.server.Close()
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return nil, fmt.Errorf("login timed out waiting for the browser callback")
	case res := <-p.results:
		if res.err != nil {
			return nil, res.err
		}
		return p.flow.exchangeCode(ctx, res.code, p.pkce)
	}
}

func (p *PendingLogin) CompleteWithPaste(raw string) error {
	code, state, err := parseOAuthPaste(raw)
	if err != nil {
		return err
	}
	if state != "" && state != p.state {
		return fmt.Errorf("state mismatch in OAuth callback (possible CSRF)")
	}
	if code == "" {
		return fmt.Errorf("callback missing authorization code")
	}
	return p.deliver(callbackResult{code: code})
}

func parseOAuthPaste(raw string) (code, state string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", fmt.Errorf("empty paste")
	}

	if !isOAuthCallbackPaste(raw) {
		return raw, "", nil
	}

	u, perr := url.Parse(raw)
	if perr != nil {
		return "", "", fmt.Errorf("invalid callback URL")
	}
	q := u.Query()
	if len(q) == 0 {
		if idx := strings.Index(raw, "?"); idx >= 0 {
			if q2, qerr := url.ParseQuery(raw[idx+1:]); qerr == nil {
				q = q2
			}
		} else if strings.Contains(raw, "code=") {
			if q2, qerr := url.ParseQuery(raw); qerr == nil {
				q = q2
			}
		}
	}

	if msg := q.Get("error"); msg != "" {
		if d := q.Get("error_description"); d != "" {
			msg = d
		}
		return "", "", fmt.Errorf("authorization failed: %s", msg)
	}
	return q.Get("code"), q.Get("state"), nil
}

func isOAuthCallbackPaste(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	if u, err := url.Parse(s); err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != "" {
		return true
	}
	if strings.Contains(s, "code=") {
		return true
	}
	return false
}

func (f FlowConfig) Login(ctx context.Context) (*Tokens, error) {
	pending, err := f.Begin()
	if err != nil {
		return nil, err
	}
	if !pending.LoopbackListening() {
		return nil, fmt.Errorf("cannot bind %s (is another login flow or CLI using it?): paste-based login is available in the TUI", f.redirectURI())
	}
	fmt.Println("Opening browser for login…")
	fmt.Println("If it does not open, visit:\n\n  " + pending.URL + "\n")
	return pending.Wait(ctx)
}

func (f FlowConfig) exchangeCode(ctx context.Context, code string, pkce pkceCodes) (*Tokens, error) {
	resp, err := postForm(ctx, f.TokenURL, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {f.redirectURI()},
		"client_id":     {f.ClientID},
		"code_verifier": {pkce.verifier},
	})
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	return resp.toTokens(""), nil
}

func (f FlowConfig) Refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {f.ClientID},
	}
	if f.RedirectHost != "" && f.RedirectPort > 0 && f.RedirectPath != "" {
		form.Set("redirect_uri", f.redirectURI())
	}
	resp, err := postForm(ctx, f.TokenURL, form)
	if err != nil {
		return nil, fmt.Errorf("token refresh: %w", err)
	}
	return resp.toTokens(refreshToken), nil
}

func postForm(ctx context.Context, endpoint string, form url.Values) (*tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	var resp tokenResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("bad token endpoint response (%s): %.200s", httpResp.Status, body)
	}
	if resp.Error != "" {
		detail := resp.ErrorDesc
		if detail == "" {
			detail = resp.Error
		}
		return &resp, fmt.Errorf("%s", detail)
	}
	if httpResp.StatusCode != http.StatusOK {
		return &resp, fmt.Errorf("unexpected status %s: %.200s", httpResp.Status, body)
	}
	return &resp, nil
}

func writeCallbackPage(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<html><body style=\"font-family: sans-serif; padding: 2rem\"><h2>mr-reviewer</h2><p>%s</p></body></html>", message)
}

var openBrowser = openBrowserDefault

func openBrowserDefault(target string) {
	if testing.Testing() {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	_ = cmd.Start()
}
