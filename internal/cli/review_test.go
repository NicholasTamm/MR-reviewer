package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

func githubFixture(t *testing.T, posted *bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/pulls/1") && !strings.Contains(r.URL.Path, "/files"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"title": "Add sys", "body": "desc",
				"html_url": "https://github.com/owner/repo/pull/1",
				"head":     map[string]string{"sha": "abc", "ref": "feature"},
				"base":     map[string]string{"ref": "main"},
			})
		case strings.Contains(r.URL.Path, "/files"):
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"filename": "hello.py", "status": "modified",
				"patch": "@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
			}})
		case strings.Contains(r.URL.Path, "/contents/"):
			w.Write([]byte("import os\nimport sys\n"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reviews"):
			if posted != nil {
				*posted = true
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestParseReviewArgsDefaultDryRun(t *testing.T) {
	got, err := ParseReviewArgs([]string{"https://github.com/o/r/pull/1", "--provider", "echo"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.DryRun || got.Post {
		t.Fatalf("%+v", got)
	}
	got, err = ParseReviewArgs([]string{"https://github.com/o/r/pull/1", "--post"})
	if err != nil || got.DryRun || !got.Post {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestRunReviewJSONDryRunTwice(t *testing.T) {
	var posted bool
	srv := githubFixture(t, &posted)
	defer srv.Close()

	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("MR_REVIEWER_GITHUB_API", srv.URL)
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("MR_REVIEWER_AUTH", authPath)
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	st, err := auth.OpenStore(authPath)
	if err != nil {
		t.Fatal(err)
	}
	target, err := auth.NewPlatformTarget("github", "https://github.com", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: "test-token"}); err != nil {
		t.Fatal(err)
	}

	argv := []string{"https://github.com/owner/repo/pull/1", "--provider", "echo", "--dry-run"}
	var out1, out2, err1, err2 bytes.Buffer
	if code := RunReview(context.Background(), argv, &out1, &err1); code != 0 {
		t.Fatalf("run1 exit %d stderr=%s stdout=%s", code, err1.String(), out1.String())
	}
	if posted {
		t.Fatal("dry-run posted")
	}
	if code := RunReview(context.Background(), argv, &out2, &err2); code != 0 {
		t.Fatalf("run2 exit %d stderr=%s", code, err2.String())
	}
	if posted {
		t.Fatal("second dry-run posted")
	}

	var r1, r2 review.Result
	if err := json.Unmarshal(out1.Bytes(), &r1); err != nil {
		t.Fatalf("json1: %v raw=%s", err, out1.String())
	}
	if err := json.Unmarshal(out2.Bytes(), &r2); err != nil {
		t.Fatalf("json2: %v", err)
	}
	if r1.Summary == "" || r2.Summary == "" {
		t.Fatalf("empty summary: %q / %q", r1.Summary, r2.Summary)
	}
	if len(r1.Comments) == 0 {
		t.Fatalf("no comments: %+v", r1)
	}
	for _, c := range r1.Comments {
		if c.File == "" || c.Line == 0 || c.Body == "" || c.Severity == "" {
			t.Fatalf("comment missing fields: %+v", c)
		}
	}
	if r1.Summary != r2.Summary || len(r1.Comments) != len(r2.Comments) {
		t.Fatalf("inconsistent runs:\n%+v\n%+v", r1, r2)
	}
}

func TestRunReviewGitHubEnterpriseUsesConfiguredAPI(t *testing.T) {
	var posted bool
	srv := githubFixture(t, &posted)
	defer srv.Close()
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("MR_REVIEWER_GITHUB_API", srv.URL)
	t.Setenv("MR_REVIEWER_HOME", t.TempDir())
	authPath := filepath.Join(t.TempDir(), "auth.json")
	t.Setenv("MR_REVIEWER_AUTH", authPath)
	st, err := auth.OpenStore(authPath)
	if err != nil {
		t.Fatal(err)
	}
	target, err := auth.NewPlatformTarget("github", "https://ghe.example.com", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: "test-token"}); err != nil {
		t.Fatal(err)
	}
	argv := []string{"https://ghe.example.com/owner/repo/pull/1", "--provider", "echo", "--dry-run"}
	var out1, out2, err1, err2 bytes.Buffer
	if code := RunReview(context.Background(), argv, &out1, &err1); code != 0 {
		t.Fatalf("ghe1 exit %d stderr=%s stdout=%s", code, err1.String(), out1.String())
	}
	if posted {
		t.Fatal("dry-run posted")
	}
	if code := RunReview(context.Background(), argv, &out2, &err2); code != 0 {
		t.Fatalf("ghe2 exit %d %s", code, err2.String())
	}
	var r1, r2 review.Result
	if err := json.Unmarshal(out1.Bytes(), &r1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out2.Bytes(), &r2); err != nil {
		t.Fatal(err)
	}
	if r1.Summary == "" || r1.Summary != r2.Summary || len(r1.Comments) == 0 {
		t.Fatalf("inconsistent %+v / %+v", r1, r2)
	}
}

func TestRunReviewFailureNonZero(t *testing.T) {
	t.Setenv("MR_REVIEWER_AUTH", filepath.Join(t.TempDir(), "auth.json"))
	var out, errb bytes.Buffer
	code := RunReview(context.Background(), []string{"https://example.com/not-a-pr", "--provider", "echo"}, &out, &errb)
	if code == 0 {
		t.Fatal("expected non-zero")
	}
	if !strings.Contains(errb.String(), "unsupported") && !strings.Contains(out.String(), "error") {
		t.Fatalf("stdout=%s stderr=%s", out.String(), errb.String())
	}
}

func TestParseReviewArgsErrorsAndEquals(t *testing.T) {
	if _, err := ParseReviewArgs(nil); err == nil {
		t.Fatal("missing url")
	}
	if _, err := ParseReviewArgs([]string{"https://github.com/o/r/pull/1", "--post", "--dry-run"}); err == nil || !strings.Contains(err.Error(), "cannot combine") {
		t.Fatalf("err = %v", err)
	}
	got, err := ParseReviewArgs([]string{"--provider=echo", "--focus=bugs,style", "--max-comments=3", "https://github.com/o/r/pull/2"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != "echo" || got.URL != "https://github.com/o/r/pull/2" || got.MaxComments != 3 {
		t.Fatalf("%+v", got)
	}
	if len(got.Focus) != 2 || got.Focus[0] != "bugs" {
		t.Fatalf("focus = %v", got.Focus)
	}
}

func TestUsageMentionsHeadless(t *testing.T) {
	u := Usage()
	if !strings.Contains(u, "review <url>") || !strings.Contains(u, "--dry-run") || !strings.Contains(u, "--config") || !strings.Contains(u, "serve") {
		t.Fatalf("%s", u)
	}
	_ = os.Stdout
}
