package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

func TestLiveSessionCatalogUsesActivePlatformAndProjectScope(t *testing.T) {
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		switch r.URL.Path {
		case "/user/repos":
			_, _ = w.Write([]byte(`[{"name":"repo","full_name":"owner/repo","html_url":"https://github.com/owner/repo"}]`))
		case "/repos/owner/repo/pulls":
			_, _ = w.Write([]byte(`[{"number":7,"title":"GitHub review","html_url":"https://github.com/owner/repo/pull/7","user":{"login":"octo"},"head":{"ref":"feature"},"base":{"ref":"main"}}]`))
		case "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":42,"path_with_namespace":"group/project","web_url":"https://gitlab.example/group/project"}]`))
		case "/api/v4/projects/42/merge_requests":
			_, _ = w.Write([]byte(`[{"iid":3,"title":"GitLab review","web_url":"https://gitlab.example/group/project/-/merge_requests/3","author":{"name":"sam"}}]`))
		default:
			t.Errorf("unexpected catalog request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Settings{GitHubAPI: server.URL, GitLabURL: server.URL, AllowInsecureGitLab: true}
	github, _ := auth.NewPlatformTarget("github", "https://github.com", server.URL)
	gitlab, _ := auth.NewPlatformTarget("gitlab", server.URL, server.URL+"/api/v4")
	for _, target := range []auth.PlatformTarget{github, gitlab} {
		if err := store.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: "test-token"}); err != nil {
			t.Fatal(err)
		}
	}
	session := &liveSession{cfg: cfg, store: store}

	projects, err := session.loadProjects("github")
	if err != nil || len(projects) != 1 || projects[0].Platform != "github" {
		t.Fatalf("github projects = %#v, err = %v", projects, err)
	}
	if _, err := session.loadReviews("github", projects[0]); err != nil {
		t.Fatal(err)
	}
	projects, err = session.loadProjects("gitlab")
	if err != nil || len(projects) != 1 || projects[0].Platform != "gitlab" {
		t.Fatalf("gitlab projects = %#v, err = %v", projects, err)
	}
	if _, err := session.loadReviews("gitlab", review.Project{Platform: "gitlab", ID: "42", Path: "group/project"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/user/repos", "/repos/owner/repo/pulls", "/api/v4/projects", "/api/v4/projects/42/merge_requests"} {
		found := false
		for _, got := range requests {
			found = found || got == want
		}
		if !found {
			t.Errorf("missing scoped request %s; got %v", want, requests)
		}
	}
}

func TestLiveSessionCatalogRequiresActivePlatformCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("catalog request made without credentials")
	}))
	defer server.Close()
	session := &liveSession{cfg: config.Settings{GitHubAPI: server.URL}}
	if _, err := session.loadProjects("github"); err == nil || err.Error() != "github browsing unavailable: platform credentials require re-login" {
		t.Fatalf("credential error = %v", err)
	}
}
