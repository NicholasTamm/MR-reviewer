package cli

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanung/mr-reviewer/internal/auth"
)

func withAuthStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("MR_REVIEWER_AUTH", path)
	for _, k := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY",
		"GEMINI_API_KEY", "GOOGLE_API_KEY", "KIMI_API_KEY", "DEEPSEEK_API_KEY",
		"GITLAB_TOKEN", "GITHUB_TOKEN",
	} {
		t.Setenv(k, "")
	}
	return path
}

func stubPrompt(t *testing.T, key string) {
	t.Helper()
	prev := promptSecret
	promptSecret = func(io.Writer, string) (string, error) {
		return key, nil
	}
	t.Cleanup(func() { promptSecret = prev })
}

func TestRunAuthStatusAndLogout(t *testing.T) {
	path := withAuthStore(t)
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("anthropic", auth.Credential{Type: auth.TypeAPIKey, APIKey: "sk-ant"}); err != nil {
		t.Fatal(err)
	}

	var out, errb bytes.Buffer
	if code := RunAuth([]string{"status"}, &out, &errb); code != 0 {
		t.Fatalf("status exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "anthropic") || !strings.Contains(out.String(), "API key stored") {
		t.Fatalf("status = %q", out.String())
	}
	if !strings.Contains(out.String(), "gitlab") || !strings.Contains(out.String(), "github") {
		t.Fatalf("status missing platforms: %q", out.String())
	}

	out.Reset()
	if code := RunAuth([]string{"logout", "anthropic"}, &out, &errb); code != 0 {
		t.Fatalf("logout exit %d stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Logged out of anthropic") {
		t.Fatalf("logout = %q", out.String())
	}
	st2, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st2.Get("anthropic"); ok {
		t.Fatal("anthropic still stored after logout")
	}
}

func TestRunAuthLogoutGeminiAliasAndMissingArg(t *testing.T) {
	path := withAuthStore(t)
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set("gemini", auth.Credential{Type: auth.TypeAPIKey, APIKey: "g"}); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"logout"}, &out, &errb); code == 0 {
		t.Fatal("logout without provider should fail")
	}
	if !strings.Contains(errb.String(), "logout") {
		t.Fatalf("stderr = %q", errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := RunAuth([]string{"logout", "gemini"}, &out, &errb); code != 0 {
		t.Fatalf("logout gemini exit %d %s", code, errb.String())
	}
	st2, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st2.Get("google"); ok {
		t.Fatal("google/gemini still stored")
	}
}

func TestRunAuthLoginAPIKeyStores(t *testing.T) {
	path := withAuthStore(t)
	stubPrompt(t, "sk-ant-from-prompt")

	var out, errb bytes.Buffer
	if code := RunAuth([]string{"login", "anthropic"}, &out, &errb); code != 0 {
		t.Fatalf("login exit %d stderr=%s stdout=%s", code, errb.String(), out.String())
	}
	if !strings.Contains(out.String(), "Stored anthropic API key") {
		t.Fatalf("stdout = %q", out.String())
	}
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	cred, ok := st.Get("anthropic")
	if !ok || cred.APIKey != "sk-ant-from-prompt" || cred.Type != auth.TypeAPIKey {
		t.Fatalf("stored = %+v ok=%v", cred, ok)
	}
}

func TestRunAuthLoginGitLabAndGitHubAPIKeyResolves(t *testing.T) {
	path := withAuthStore(t)
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"login", "gitlab", "--api-key", "glpat-cli-token"}, &out, &errb); code != 0 {
		t.Fatalf("gitlab login exit %d stderr=%s stdout=%s", code, errb.String(), out.String())
	}
	if !strings.Contains(out.String(), "Stored gitlab API key") {
		t.Fatalf("stdout = %q", out.String())
	}
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := auth.PlatformToken("gitlab", st)
	if err != nil || tok != "glpat-cli-token" {
		t.Fatalf("PlatformToken gitlab = %q err=%v", tok, err)
	}

	out.Reset()
	errb.Reset()
	if code := RunAuth([]string{"login", "github", "--api-key=ghp-cli-token"}, &out, &errb); code != 0 {
		t.Fatalf("github login exit %d %s %s", code, errb.String(), out.String())
	}
	st, err = auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	tok, err = auth.PlatformToken("github", st)
	if err != nil || tok != "ghp-cli-token" {
		t.Fatalf("PlatformToken github = %q err=%v", tok, err)
	}
	key, ok := auth.APIKey("github", st)
	if !ok || key != "ghp-cli-token" {
		t.Fatalf("APIKey github = %q ok=%v", key, ok)
	}
}

func TestRunAuthLoginGitLabStdinPrompt(t *testing.T) {
	path := withAuthStore(t)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	if _, err := w.WriteString("glpat-from-stdin\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	var out, errb bytes.Buffer
	if code := RunAuth([]string{"login", "gitlab", "--api-key"}, &out, &errb); code != 0 {
		t.Fatalf("login exit %d stderr=%s stdout=%s", code, errb.String(), out.String())
	}
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := auth.PlatformToken("gitlab", st)
	if err != nil || tok != "glpat-from-stdin" {
		t.Fatalf("resolved = %q err=%v", tok, err)
	}
	if !strings.Contains(out.String(), "GitLab personal access token") {
		t.Fatalf("prompt missing: %q", out.String())
	}
}

func TestRunAuthGitLabEnvOverStore(t *testing.T) {
	path := withAuthStore(t)
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"login", "gitlab", "--api-key", "stored-gl"}, &out, &errb); code != 0 {
		t.Fatal(errb.String())
	}
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITLAB_TOKEN", "env-gl")
	tok, err := auth.PlatformToken("gitlab", st)
	if err != nil || tok != "env-gl" {
		t.Fatalf("env should win: %q err=%v", tok, err)
	}
}

func TestRunAuthLoginProviderAPIKeyFlag(t *testing.T) {
	path := withAuthStore(t)
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"login", "openai", "--api-key", "sk-openai"}, &out, &errb); code != 0 {
		t.Fatalf("openai --api-key: %s %s", errb.String(), out.String())
	}
	st, err := auth.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	key, ok := auth.APIKey("openai", st)
	if !ok || key != "sk-openai" {
		t.Fatalf("openai key = %q ok=%v", key, ok)
	}
}

func TestRunAuthStatusPrefersEnv(t *testing.T) {
	withAuthStore(t)
	t.Setenv("XAI_API_KEY", "from-env")
	t.Setenv("GITLAB_TOKEN", "gl-env")
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"status"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "using XAI_API_KEY from environment") {
		t.Fatalf("status = %q", out.String())
	}
	if !strings.Contains(out.String(), "using GITLAB_TOKEN from environment") {
		t.Fatalf("gitlab env status = %q", out.String())
	}
}

func TestRunAuthStatusNotLoggedIn(t *testing.T) {
	withAuthStore(t)
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"status"}, &out, &errb); code != 0 {
		t.Fatal(errb.String())
	}
	if !strings.Contains(out.String(), "not logged in") {
		t.Fatalf("status = %q", out.String())
	}
}

func TestRunAuthUnknownAndUsage(t *testing.T) {
	withAuthStore(t)
	var out, errb bytes.Buffer
	if code := RunAuth(nil, &out, &errb); code != 0 {
		t.Fatalf("empty args exit %d", code)
	}
	if !strings.Contains(out.String(), "auth status") {
		t.Fatalf("usage = %q", out.String())
	}
	out.Reset()
	if code := RunAuth([]string{"nope"}, &out, &errb); code == 0 {
		t.Fatal("unknown subcommand should fail")
	}
	if code := RunAuth([]string{"login"}, &out, &errb); code == 0 {
		t.Fatal("login without provider should fail")
	}
	if code := RunAuth([]string{"login", "not-a-vendor"}, &out, &errb); code == 0 {
		t.Fatal("unknown provider should fail")
	}
}

func TestParseAPIKeyFlag(t *testing.T) {
	used, val := parseAPIKeyFlag([]string{"--device"})
	if used || val != "" {
		t.Fatalf("%v %q", used, val)
	}
	used, val = parseAPIKeyFlag([]string{"--api-key"})
	if !used || val != "" {
		t.Fatalf("flag only: %v %q", used, val)
	}
	used, val = parseAPIKeyFlag([]string{"--api-key", "tok"})
	if !used || val != "tok" {
		t.Fatalf("flag value: %v %q", used, val)
	}
	used, val = parseAPIKeyFlag([]string{"--api-key=tok2"})
	if !used || val != "tok2" {
		t.Fatalf("equals: %v %q", used, val)
	}
}

func TestPromptSecretDefaultReadsStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	if _, err := w.WriteString("secret-line\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	var out bytes.Buffer
	got, err := promptSecretDefault(&out, "Paste: ")
	if err != nil || got != "secret-line" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if !strings.Contains(out.String(), "Paste: ") {
		t.Fatalf("prompt = %q", out.String())
	}
}
