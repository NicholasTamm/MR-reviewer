package cli

import (
	"bytes"
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
	// Isolate from a developer machine's exported keys.
	for _, k := range []string{
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "XAI_API_KEY",
		"GEMINI_API_KEY", "GOOGLE_API_KEY", "KIMI_API_KEY", "DEEPSEEK_API_KEY",
		"GITLAB_TOKEN", "GITHUB_TOKEN",
	} {
		t.Setenv(k, "")
	}
	return path
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
	if !strings.Contains(out.String(), "anthropic") || !strings.Contains(out.String(), "api key") {
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
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old })
	if _, err := w.WriteString("sk-ant-from-stdin\n"); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

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
	if !ok || cred.APIKey != "sk-ant-from-stdin" || cred.Type != auth.TypeAPIKey {
		t.Fatalf("stored = %+v ok=%v", cred, ok)
	}
}

func TestRunAuthStatusPrefersEnv(t *testing.T) {
	withAuthStore(t)
	t.Setenv("XAI_API_KEY", "from-env")
	var out, errb bytes.Buffer
	if code := RunAuth([]string{"status"}, &out, &errb); code != 0 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "XAI_API_KEY") {
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
