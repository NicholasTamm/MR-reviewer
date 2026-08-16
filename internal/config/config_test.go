package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/platform"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MR_REVIEWER_HOME", dir)
	t.Setenv("MR_REVIEWER_CONFIG", "")
	t.Setenv("MR_REVIEWER_PROVIDERS", "")
	return dir
}

func TestLoadDefaults(t *testing.T) {
	isolateConfig(t)
	t.Setenv("MR_REVIEWER_GITLAB_URL", "")
	t.Setenv("MR_REVIEWER_GITHUB_API", "")
	t.Setenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB", "")
	t.Setenv("MR_REVIEWER_PROVIDER", "")
	t.Setenv("MR_REVIEWER_MODEL", "")
	t.Setenv("MR_REVIEWER_FOCUS", "")
	t.Setenv("MR_REVIEWER_MAX_COMMENTS", "")
	s := Load()
	if s.GitLabURL != "https://gitlab.com" || s.GitHubAPI != "https://api.github.com" {
		t.Fatalf("urls = %+v", s)
	}
	if s.AllowInsecureGitLab || s.Provider != "anthropic" || s.Model != "" {
		t.Fatalf("defaults = %+v", s)
	}
	if s.MaxComments != 10 {
		t.Fatalf("max = %d", s.MaxComments)
	}
	if strings.Join(s.Focus, ",") != strings.Join(review.DefaultFocus, ",") {
		t.Fatalf("focus = %v", s.Focus)
	}
}

func TestLoadFromEnv(t *testing.T) {
	isolateConfig(t)
	t.Setenv("MR_REVIEWER_GITLAB_URL", "https://git.example.com/")
	t.Setenv("MR_REVIEWER_GITHUB_API", "https://github.example/api/")
	t.Setenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB", "true")
	t.Setenv("MR_REVIEWER_PROVIDER", "openai")
	t.Setenv("MR_REVIEWER_MODEL", "gpt-4o")
	t.Setenv("MR_REVIEWER_FOCUS", " security , bugs ")
	t.Setenv("MR_REVIEWER_MAX_COMMENTS", "7")
	s := Load()
	if s.GitLabURL != "https://git.example.com" || s.GitHubAPI != "https://github.example/api" {
		t.Fatalf("trimmed urls = %+v", s)
	}
	if !s.AllowInsecureGitLab || s.Provider != "openai" || s.Model != "gpt-4o" {
		t.Fatalf("env = %+v", s)
	}
	if s.MaxComments != 7 {
		t.Fatalf("max = %d", s.MaxComments)
	}
	if len(s.Focus) != 2 || s.Focus[0] != "security" || s.Focus[1] != "bugs" {
		t.Fatalf("focus = %v", s.Focus)
	}
}

func TestLoadInvalidMaxCommentsKeepsDefault(t *testing.T) {
	isolateConfig(t)
	t.Setenv("MR_REVIEWER_MAX_COMMENTS", "nope")
	if Load().MaxComments != 10 {
		t.Fatalf("max = %d", Load().MaxComments)
	}
	t.Setenv("MR_REVIEWER_MAX_COMMENTS", "0")
	if Load().MaxComments != 10 {
		t.Fatalf("zero max = %d", Load().MaxComments)
	}
	t.Setenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB", "yes")
	if !Load().AllowInsecureGitLab {
		t.Fatal("yes should be truthy")
	}
	t.Setenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB", "no")
	if Load().AllowInsecureGitLab {
		t.Fatal("no should be false")
	}
}

func TestPlatformForInsecureGitLabRefused(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GITLAB_TOKEN", "glpat-test")
	t.Setenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB", "")
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.PlatformFor(review.Info{Platform: "gitlab", BaseURL: "http://git.local"}, st)
	if err == nil || !strings.Contains(err.Error(), "insecure HTTP GitLab") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlatformForInsecureGitLabAllowed(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GITLAB_TOKEN", "glpat-test")
	t.Setenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB", "1")
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PlatformFor(review.Info{Platform: "gitlab", BaseURL: "http://git.local"}, st)
	if err != nil {
		t.Fatal(err)
	}
	gl, ok := p.(*platform.GitLab)
	if !ok || gl.BaseURL != "http://git.local" || gl.Token != "glpat-test" {
		t.Fatalf("%T %+v", p, gl)
	}
}

func TestPlatformForGitHubUsesEnvTokenAndAPI(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GITHUB_TOKEN", "ghp-test")
	t.Setenv("MR_REVIEWER_GITHUB_API", "https://ghe.example/api")
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.PlatformFor(review.Info{Platform: "github", Namespace: "o", Project: "r", IID: 1}, st)
	if err != nil {
		t.Fatal(err)
	}
	gh, ok := p.(*platform.GitHub)
	if !ok || gh.Token != "ghp-test" || gh.BaseURL != "https://ghe.example/api" {
		t.Fatalf("%+v", gh)
	}
}

func TestPlatformForMissingToken(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PlatformFor(review.Info{Platform: "github"}, st); err == nil {
		t.Fatal("expected missing github token")
	}
	if _, err := s.PlatformFor(review.Info{Platform: "unknown"}, st); err == nil || !strings.Contains(err.Error(), "unknown platform") {
		t.Fatalf("err = %v", err)
	}
}

func TestPlatformForUsesStoreToken(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GITHUB_TOKEN", "")
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("github", auth.Credential{Type: auth.TypeAPIKey, APIKey: "stored-gh"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.PlatformFor(review.Info{Platform: "github"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if p.(*platform.GitHub).Token != "stored-gh" {
		t.Fatalf("token = %q", p.(*platform.GitHub).Token)
	}
}

func TestGitLabBrowserRequiresToken(t *testing.T) {
	isolateConfig(t)
	t.Setenv("GITLAB_TOKEN", "")
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GitLabBrowser(st); err == nil {
		t.Fatal("expected missing gitlab token")
	}
	t.Setenv("GITLAB_TOKEN", "glpat")
	s = Load()
	gl, err := s.GitLabBrowser(st)
	if err != nil || gl.Token != "glpat" || gl.BaseURL != s.GitLabURL {
		t.Fatalf("gl=%+v err=%v", gl, err)
	}
}

func TestNewProviderEchoAndUnknown(t *testing.T) {
	isolateConfig(t)
	s := Settings{Provider: "echo"}
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.NewProvider("", "", st)
	if err != nil || p.Name() != "echo" {
		t.Fatalf("p=%v err=%v", p, err)
	}
	p, err = s.NewProvider("gemini", "gemini-2.5-flash", st)
	if err != nil || p.Name() != "google" {
		t.Fatalf("gemini alias p=%v err=%v", p, err)
	}
	if _, err := s.NewProvider("not-a-vendor", "", st); err == nil {
		t.Fatal("expected unknown provider")
	}
}

func TestSaveLoadEnvOverFile(t *testing.T) {
	isolateConfig(t)
	t.Setenv("MR_REVIEWER_GITHUB_API", "")
	t.Setenv("MR_REVIEWER_GITLAB_URL", "")
	t.Setenv("MR_REVIEWER_ANTHROPIC_URL", "")
	if err := Save(Settings{
		GitHubAPI:    "https://ghe.example/api/v3",
		GitLabURL:    "https://gitlab.example",
		AnthropicURL: "https://claude.example",
	}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(ConfigPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o", info.Mode().Perm())
	}
	s := Load()
	if s.GitHubAPI != "https://ghe.example/api/v3" || s.GitLabURL != "https://gitlab.example" || s.AnthropicURL != "https://claude.example" {
		t.Fatalf("file load = %+v", s)
	}
	t.Setenv("MR_REVIEWER_GITHUB_API", "https://from-env/api")
	t.Setenv("MR_REVIEWER_ANTHROPIC_URL", "https://anthropic-env")
	s = Load()
	if s.GitHubAPI != "https://from-env/api" || s.AnthropicURL != "https://anthropic-env" {
		t.Fatalf("env should win: %+v", s)
	}
	if s.GitLabURL != "https://gitlab.example" {
		t.Fatalf("gitlab file value lost: %q", s.GitLabURL)
	}
}

func TestParseProvidersJSONCCustomAndOverlay(t *testing.T) {
	raw := []byte(`
// custom proxy + builtin overlay
{
  "acme": {
    "npm": "@ai-sdk/openai-compatible",
    "options": {
      "baseURL": "https://llm.acme.example/v1",
      "apiKey": "{env:ACME_API_KEY}"
    },
    "models": ["acme-1"]
  },
  "anthropic": {
    "options": { "baseURL": "https://claude.proxy.example" }
  }
}
`)
	pf, err := ParseProvidersFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	c, ok := FindCustom(pf.Customs, "acme")
	if !ok || c.BaseURL != "https://llm.acme.example/v1" || c.API != WireOpenAI || c.APIKeyEnv != "ACME_API_KEY" {
		t.Fatalf("custom = %+v ok=%v", c, ok)
	}
	ep, ok := pf.Endpoints["anthropic"]
	if !ok || ep.BaseURL != "https://claude.proxy.example" {
		t.Fatalf("overlay = %+v ok=%v", ep, ok)
	}
}

func TestNewProviderCustomAndAnthropicOverlayHTTPTest(t *testing.T) {
	isolateConfig(t)
	t.Setenv("MR_REVIEWER_ANTHROPIC_URL", "")
	t.Setenv("ACME_API_KEY", "acme-secret")
	t.Setenv("ANTHROPIC_API_KEY", "ant-secret")

	var customPath, anthPath string
	customSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		customPath = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer acme-secret" {
			t.Errorf("custom auth = %q", r.Header.Get("Authorization"))
		}
		args, _ := json.Marshal(map[string]any{
			"summary": "custom-ok", "comments": []any{},
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"tool_calls": []map[string]any{{
						"function": map[string]any{"name": "submit_review", "arguments": string(args)},
					}},
				},
			}},
		})
	}))
	defer customSrv.Close()
	anthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthPath = r.URL.Path
		if r.Header.Get("x-api-key") != "ant-secret" {
			t.Errorf("anth key = %q", r.Header.Get("x-api-key"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{
				"type": "tool_use", "name": "submit_review",
				"input": map[string]any{"summary": "anth-ok", "comments": []any{}},
			}},
		})
	}))
	defer anthSrv.Close()

	body := `{
  "acme": { "api": "openai", "options": { "baseURL": "` + customSrv.URL + `", "apiKey": "{env:ACME_API_KEY}" } },
  "anthropic": { "options": { "baseURL": "` + anthSrv.URL + `" } }
}`
	if err := os.WriteFile(filepath.Join(HomeDir(), "providers.jsonc"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s := Load()
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}

	cp, err := s.NewProvider("acme", "acme-1", st)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cp.Review(context.Background(), "sys", "user")
	if err != nil || got.Summary != "custom-ok" {
		t.Fatalf("custom review = %+v err=%v", got, err)
	}
	if !strings.Contains(customPath, "chat/completions") {
		t.Fatalf("custom path = %q", customPath)
	}

	ap, err := s.NewProvider("anthropic", "claude-test", st)
	if err != nil {
		t.Fatal(err)
	}
	got, err = ap.Review(context.Background(), "sys", "user")
	if err != nil || got.Summary != "anth-ok" {
		t.Fatalf("anth review = %+v err=%v", got, err)
	}
	if !strings.Contains(anthPath, "messages") {
		t.Fatalf("anth path = %q", anthPath)
	}
}
