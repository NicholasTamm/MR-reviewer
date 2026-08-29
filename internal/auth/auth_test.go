package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreRoundTripAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	cred := Credential{Type: TypeOAuth, Access: "acc", Refresh: "ref", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := st.Set("xai", cred); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("auth.json permissions = %o, want 600", perm)
	}
	st2, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := st2.Get("xai")
	if !ok || got.Access != "acc" || got.Refresh != "ref" {
		t.Errorf("reloaded credential = %+v, ok=%v", got, ok)
	}
}

func TestDefaultPathUsesMrReviewerHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MR_REVIEWER_AUTH", "")
	t.Setenv("MR_REVIEWER_HOME", "")
	got := DefaultPath()
	want := filepath.Join(home, ".mr-reviewer", "auth.json")
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestDefaultPathUsesSharedMrReviewerHomeOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MR_REVIEWER_AUTH", "")
	t.Setenv("MR_REVIEWER_HOME", root)
	if got, want := DefaultPath(), filepath.Join(root, "auth.json"); got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}

func TestDefaultPathEnvOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "custom.json")
	t.Setenv("MR_REVIEWER_AUTH", p)
	if got := DefaultPath(); got != p {
		t.Errorf("DefaultPath = %q, want %q", got, p)
	}
}

func TestStoreDeleteAndProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Set("openai", Credential{Type: TypeAPIKey, APIKey: "k1"})
	_ = st.Set("xai", Credential{Type: TypeAPIKey, APIKey: "k2"})
	_ = st.Set("anthropic", Credential{Type: TypeAPIKey, APIKey: "k3"})
	if got := st.Providers(); !reflect.DeepEqual(got, []string{"anthropic", "openai", "xai"}) {
		t.Errorf("Providers = %v", got)
	}
	_ = st.Delete("openai")
	if _, ok := st.Get("openai"); ok {
		t.Error("openai still present")
	}
}

func TestGoogleStoreCanonicalAndLegacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"gemini":{"type":"api","apiKey":"legacy-key"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"google", "gemini"} {
		cred, ok := st.Get(id)
		if !ok || cred.APIKey != "legacy-key" {
			t.Fatalf("Get(%q) = %+v ok=%v", id, cred, ok)
		}
	}
	if got := st.Providers(); !reflect.DeepEqual(got, []string{"google"}) {
		t.Errorf("Providers = %v", got)
	}
	if err := st.Set("gemini", Credential{Type: TypeAPIKey, APIKey: "from-alias"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var disk map[string]Credential
	_ = json.Unmarshal(raw, &disk)
	if _, ok := disk["gemini"]; ok {
		t.Errorf("gemini still on disk: %s", raw)
	}
	if c, ok := disk["google"]; !ok || c.APIKey != "from-alias" {
		t.Errorf("disk google = %+v", c)
	}
}

func TestAPIKeyEnvOverStore(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "")
	if _, ok := APIKey("openai", st); ok {
		t.Fatal("expected no key")
	}
	if err := st.Set("openai", Credential{Type: TypeAPIKey, APIKey: "stored"}); err != nil {
		t.Fatal(err)
	}
	if key, ok := APIKey("openai", st); !ok || key != "stored" {
		t.Errorf("key=%q ok=%v", key, ok)
	}
	t.Setenv("OPENAI_API_KEY", "from-env")
	if key, ok := APIKey("openai", st); !ok || key != "from-env" {
		t.Errorf("key=%q ok=%v", key, ok)
	}
}

func TestAPIKeyGoogleEnvNames(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "google-ai-studio-key")
	for _, id := range []string{"google", "gemini"} {
		if key, ok := APIKey(id, st); !ok || key != "google-ai-studio-key" {
			t.Errorf("GOOGLE_API_KEY via %q: key=%q ok=%v", id, key, ok)
		}
	}
	t.Setenv("GEMINI_API_KEY", "primary-gemini-key")
	for _, id := range []string{"google", "gemini"} {
		if key, ok := APIKey(id, st); !ok || key != "primary-gemini-key" {
			t.Errorf("GEMINI_API_KEY via %q: %q", id, key)
		}
		if got := Describe(id, st); got != "GEMINI_API_KEY" {
			t.Errorf("Describe(%q) = %q", id, got)
		}
	}
}

func TestBearerSourcePrecedence(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	t.Setenv("XAI_API_KEY", "")
	if _, err := BearerSource("xai", st)(ctx); err == nil {
		t.Error("expected error")
	}
	_ = st.Set("xai", Credential{Type: TypeOAuth, Access: "oauth-token", Refresh: "r", ExpiresAt: time.Now().Add(time.Hour)})
	if got, err := BearerSource("xai", st)(ctx); err != nil || got != "oauth-token" {
		t.Errorf("bearer = %q err=%v", got, err)
	}
	_ = st.Set("xai", Credential{Type: TypeOAuth, Access: "oauth-token", APIKey: "stored-key", ExpiresAt: time.Now().Add(time.Hour)})
	if got, _ := BearerSource("xai", st)(ctx); got != "stored-key" {
		t.Errorf("bearer = %q", got)
	}
	t.Setenv("XAI_API_KEY", "env-key")
	if got, _ := BearerSource("xai", st)(ctx); got != "env-key" {
		t.Errorf("bearer = %q", got)
	}
}

func TestCompleteOpenAILoginPersistsExchangedAPIKey(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	previous := exchangeOpenAIAPIKey
	exchangeOpenAIAPIKey = func(_ context.Context, accessToken string) (string, error) {
		if accessToken != "oauth-access" {
			t.Fatalf("access token = %q", accessToken)
		}
		return "exchanged-key", nil
	}
	t.Cleanup(func() { exchangeOpenAIAPIKey = previous })

	message, err := CompleteLogin(context.Background(), st, "openai", &Tokens{Access: "oauth-access", Refresh: "refresh", IDToken: "id-token", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || !strings.Contains(message, "ready for API-backed reviews") {
		t.Fatalf("message=%q err=%v", message, err)
	}
	credential, ok := st.Get("openai")
	if !ok || credential.APIKey != "exchanged-key" || credential.Access != "oauth-access" {
		t.Fatalf("credential=%+v ok=%v", credential, ok)
	}
	if key, err := BearerSource("openai", st)(context.Background()); err != nil || key != "exchanged-key" {
		t.Fatalf("key=%q err=%v", key, err)
	}
}

func TestCompleteOpenAILoginExchangeFailureDoesNotPersistOAuthToken(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	previous := exchangeOpenAIAPIKey
	exchangeOpenAIAPIKey = func(context.Context, string) (string, error) { return "", fmt.Errorf("exchange unavailable") }
	t.Cleanup(func() { exchangeOpenAIAPIKey = previous })

	_, err = CompleteLogin(context.Background(), st, "openai", &Tokens{Access: "oauth-access", IDToken: "id-token"})
	if err == nil || !strings.Contains(err.Error(), "could not obtain an API key") {
		t.Fatalf("err=%v", err)
	}
	if _, ok := st.Get("openai"); ok {
		t.Fatal("must not persist an OAuth token that cannot be used for OpenAI API calls")
	}
}

func TestBearerSourceOpenAIRejectsMissingOAuthAccessToken(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("openai", Credential{Type: TypeOAuth, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	key, err := BearerSource("openai", st)(context.Background())
	if err == nil || key != "" || !strings.Contains(err.Error(), "requires a standard API key") {
		t.Fatalf("key=%q err=%v", key, err)
	}
}

func TestBearerSourceOpenAIExchangesAfterRefresh(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("openai", Credential{Type: TypeOAuth, Access: "old-access", Refresh: "refresh", IDToken: "old-id", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "refresh_token": "new-refresh", "id_token": "new-id", "expires_in": 3600})
	}))
	defer refreshServer.Close()
	previousFlow := refreshFlows["openai"]
	refreshFlows["openai"] = FlowConfig{TokenURL: refreshServer.URL, ClientID: "test-client"}
	t.Cleanup(func() { refreshFlows["openai"] = previousFlow })
	previousExchange := exchangeOpenAIAPIKey
	exchangeOpenAIAPIKey = func(_ context.Context, accessToken string) (string, error) {
		if accessToken != "new-access" {
			t.Fatalf("access token = %q", accessToken)
		}
		return "refreshed-api-key", nil
	}
	t.Cleanup(func() { exchangeOpenAIAPIKey = previousExchange })

	key, err := BearerSource("openai", st)(context.Background())
	if err != nil || key != "refreshed-api-key" {
		t.Fatalf("key=%q err=%v", key, err)
	}
	credential, _ := st.Get("openai")
	if credential.APIKey != "refreshed-api-key" || credential.Access != "new-access" {
		t.Fatalf("credential=%+v", credential)
	}
}

func TestConcurrentProviderRefreshUsesOneRequestAndPersistsRotation(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("xai", Credential{Type: TypeOAuth, Access: "old-access", Refresh: "old-refresh", ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	var requests int
	started := make(chan struct{})
	release := make(chan struct{})
	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.FormValue("refresh_token"); got != "old-refresh" {
			t.Errorf("refresh_token = %q", got)
		}
		requests++
		close(started)
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new-access", "refresh_token": "new-refresh", "expires_in": 3600})
	}))
	defer refreshServer.Close()
	previousFlow := refreshFlows["xai"]
	refreshFlows["xai"] = FlowConfig{TokenURL: refreshServer.URL, ClientID: "test-client"}
	t.Cleanup(func() { refreshFlows["xai"] = previousFlow })

	const callers = 8
	results := make(chan string, callers)
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := BearerSource("xai", st)(context.Background())
			if err != nil {
				errs <- err
				return
			}
			results <- token
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if requests != 1 {
		t.Errorf("refresh requests = %d, want 1", requests)
	}
	for token := range results {
		if token != "new-access" {
			t.Errorf("token = %q", token)
		}
	}
	credential, ok := st.Get("xai")
	if !ok || credential.Access != "new-access" || credential.Refresh != "new-refresh" {
		t.Fatalf("persisted credential = %+v, ok=%v", credential, ok)
	}
}

func TestRefreshFlowsNoGoogleOAuth(t *testing.T) {
	for _, id := range []string{"google", "gemini"} {
		if _, ok := refreshFlows[id]; ok {
			t.Fatalf("%s must not have OAuth refresh", id)
		}
	}
	if _, ok := refreshFlows["openai"]; !ok {
		t.Fatal("openai refresh missing")
	}
	if _, ok := refreshFlows["xai"]; !ok {
		t.Fatal("xai refresh missing")
	}
}

func TestNewPKCE(t *testing.T) {
	a := newPKCE()
	b := newPKCE()
	if a.verifier == "" || a.challenge == "" || a.verifier == b.verifier {
		t.Fatalf("%+v vs %+v", a, b)
	}
}

func TestDescribe(t *testing.T) {
	st, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XAI_API_KEY", "")
	if got := Describe("xai", st); got != "none" {
		t.Errorf("empty = %q", got)
	}
	_ = st.Set("xai", Credential{Type: TypeAPIKey, APIKey: "k"})
	if got := Describe("xai", st); got != "api key" {
		t.Errorf("api = %q", got)
	}
	_ = st.Set("xai", Credential{Type: TypeOAuth, Access: "a", APIKey: "k"})
	if got := Describe("xai", st); got != "oauth+key" {
		t.Errorf("oauth+key = %q", got)
	}
	t.Setenv("XAI_API_KEY", "env")
	if got := Describe("xai", st); got != "XAI_API_KEY" {
		t.Errorf("env = %q", got)
	}
}

func TestFlowConfigShapes(t *testing.T) {
	o := OpenAIFlow()
	if o.ClientID != openaiClientID || o.RedirectPort != 1455 || o.RedirectHost != "localhost" {
		t.Errorf("OpenAIFlow = %+v", o)
	}
	if !strings.Contains(o.AuthorizeURL, "auth.openai.com") || o.RedirectPath != "/auth/callback" {
		t.Errorf("OpenAI authorize = %q path=%q", o.AuthorizeURL, o.RedirectPath)
	}
	x := XAIFlow()
	if x.ClientID != xaiClientID || x.RedirectPort != 56121 || x.RedirectHost != "127.0.0.1" || !x.IncludeNonce {
		t.Errorf("XAIFlow = %+v", x)
	}
	if x.ExtraParams["plan"] != "generic" {
		t.Errorf("ExtraParams = %v", x.ExtraParams)
	}
	d := XAIDeviceFlow()
	if d.ClientID != xaiClientID || !strings.Contains(d.DeviceURL, "/oauth2/device/code") {
		t.Errorf("XAIDeviceFlow = %+v", d)
	}
}

func TestGitLabFlowRequiresExplicitClientID(t *testing.T) {
	if _, err := GitLabFlow(""); err == nil || !strings.Contains(err.Error(), "GITLAB_OAUTH_CLIENT_ID") {
		t.Fatalf("missing client ID error = %v", err)
	}
	flow, err := GitLabFlow("client-id")
	if err != nil {
		t.Fatal(err)
	}
	if flow.ClientID != "client-id" || flow.Scope != "api" || flow.RedirectHost != "127.0.0.1" || flow.RedirectPort != 8620 || flow.RedirectPath != "/oauth/callback" {
		t.Fatalf("GitLabFlow = %+v", flow)
	}
	device, err := GitLabDeviceFlow("client-id")
	if err != nil || device.ClientID != "client-id" || device.Scope != "api" || !strings.Contains(device.DeviceURL, "authorize_device") {
		t.Fatalf("GitLabDeviceFlow = %+v err=%v", device, err)
	}
}

func TestGitLabOAuthClientIDUsesDefaultAndOverride(t *testing.T) {
	t.Setenv(gitLabOAuthClientIDEnv, "")
	if got := GitLabOAuthClientID(); got != gitLabOAuthClientID {
		t.Fatalf("default client ID = %q", got)
	}
	t.Setenv(gitLabOAuthClientIDEnv, "custom-client-id")
	if got := GitLabOAuthClientID(); got != "custom-client-id" {
		t.Fatalf("override client ID = %q", got)
	}
}

func TestGitHubDeviceFlowRequiresExplicitClientIDAndRepoScope(t *testing.T) {
	if _, err := GitHubDeviceFlow(""); err == nil || !strings.Contains(err.Error(), "GITHUB_OAUTH_CLIENT_ID") || !strings.Contains(err.Error(), "Device Flow") {
		t.Fatalf("missing client ID error = %v", err)
	}
	flow, err := GitHubDeviceFlow(" client-id ")
	if err != nil {
		t.Fatal(err)
	}
	if flow.ClientID != "client-id" || flow.Scope != "repo" || flow.DeviceURL != "https://github.com/login/device/code" || flow.TokenURL != "https://github.com/login/oauth/access_token" {
		t.Fatalf("GitHubDeviceFlow = %+v", flow)
	}
}

func TestGitHubOAuthClientIDUsesDefaultAndOverride(t *testing.T) {
	t.Setenv(gitHubOAuthClientIDEnv, "")
	if got := GitHubOAuthClientID(); got != gitHubOAuthClientID {
		t.Fatalf("default client ID = %q", got)
	}
	t.Setenv(gitHubOAuthClientIDEnv, "custom-client-id")
	if got := GitHubOAuthClientID(); got != "custom-client-id" {
		t.Fatalf("override client ID = %q", got)
	}
}

func TestValidateGitHubScopes(t *testing.T) {
	if err := ValidateGitHubScopes(&Tokens{Scope: "repo,gist"}); err != nil {
		t.Fatalf("repo scope rejected: %v", err)
	}
	if err := ValidateGitHubScopes(&Tokens{Scope: "public_repo"}); err == nil || !strings.Contains(err.Error(), "repo scope") {
		t.Fatalf("unexpected validation error = %v", err)
	}
}
