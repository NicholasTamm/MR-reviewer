package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

func TestGitHubFetchAndDryPost(t *testing.T) {
	var posted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls/1":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"title":    "Add sys",
				"body":     "desc",
				"html_url": "https://github.com/owner/repo/pull/1",
				"head":     map[string]string{"sha": "abc123", "ref": "feature"},
				"base":     map[string]string{"ref": "main"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/pulls/1/files":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"filename": "main.py",
				"status":   "modified",
				"patch":    "@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
			}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/repos/owner/repo/contents/main.py"):
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("import os\nimport sys\n"))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/owner/repo/pulls/1/reviews":
			posted = true
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if payload["commit_id"] != "abc123" {
				t.Errorf("commit_id = %v", payload["commit_id"])
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &GitHub{BaseURL: srv.URL, Token: "tok"}
	info := review.Info{Platform: "github", Host: "github.com", Namespace: "owner", Project: "repo", IID: 1}
	fr, err := c.FetchChanges(context.Background(), info)
	if err != nil {
		t.Fatal(err)
	}
	if fr.Metadata.Title != "Add sys" || fr.Metadata.SourceBranch != "feature" || len(fr.DiffFiles) != 1 {
		t.Fatalf("%+v", fr)
	}
	content, ok, err := c.FetchFile(context.Background(), info, "main.py", "feature")
	if err != nil || !ok || !strings.Contains(content, "import sys") {
		t.Fatalf("file = %q ok=%v err=%v", content, ok, err)
	}
	if err := c.PostReview(context.Background(), info, review.Result{
		Summary:  "Looks good.",
		Comments: []review.Comment{{File: "main.py", Line: 2, Body: "n", Severity: "info", IsNewLine: true}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if !posted {
		t.Fatal("expected post")
	}
}

func TestGitHubFetchErrorAndMissingFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/contents/") {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	c := &GitHub{BaseURL: srv.URL, Token: "tok"}
	info := review.Info{Namespace: "owner", Project: "repo", IID: 9}
	_, err := c.FetchChanges(context.Background(), info)
	if err == nil || !strings.Contains(err.Error(), "failed to fetch PR") {
		t.Fatalf("err = %v", err)
	}
	body, ok, err := c.FetchFile(context.Background(), info, "gone.py", "main")
	if err != nil || ok || body != "" {
		t.Fatalf("missing file body=%q ok=%v err=%v", body, ok, err)
	}
}

func TestGitHubPostRequiresFetch(t *testing.T) {
	c := &GitHub{BaseURL: "http://127.0.0.1:1"}
	err := c.PostReview(context.Background(), review.Info{Namespace: "o", Project: "r", IID: 1}, review.Result{Summary: "x"}, nil)
	if err == nil || !strings.Contains(err.Error(), "FetchChanges") {
		t.Fatalf("err = %v", err)
	}
}
