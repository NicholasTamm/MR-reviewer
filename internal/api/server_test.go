package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/platform"
	"github.com/jonathanung/mr-reviewer/internal/provider"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

type fakePlatform struct {
	fetch   review.FetchResult
	files   map[string]string
	posted  *review.Result
	postErr error
	info    review.Info
}

func (f *fakePlatform) FetchChanges(_ context.Context, info review.Info) (review.FetchResult, error) {
	f.info = info
	return f.fetch, nil
}
func (f *fakePlatform) FetchFile(_ context.Context, _ review.Info, path, _ string) (string, bool, error) {
	c, ok := f.files[path]
	return c, ok, nil
}
func (f *fakePlatform) PostReview(_ context.Context, _ review.Info, result review.Result, _ []review.DiffLine) error {
	cp := result
	f.posted = &cp
	return f.postErr
}

type fakeProvider struct {
	result review.Result
	err    error
	calls  int
}

func (f *fakeProvider) Name() string { return "fake" }
func (f *fakeProvider) Review(context.Context, string, string) (review.Result, error) {
	f.calls++
	return f.result, f.err
}

type fakeGitLab struct {
	projects []review.ProjectSummary
	grouped  []review.ProjectMergeRequests
	scoped   review.ProjectMergeRequests
	lastID   int
	err      error
}

func (f *fakeGitLab) ListVisibleMergeRequests(context.Context, string) ([]review.ProjectMergeRequests, error) {
	return f.grouped, f.err
}
func (f *fakeGitLab) ListVisibleProjects(context.Context, string) ([]review.ProjectSummary, error) {
	return f.projects, f.err
}
func (f *fakeGitLab) ListProjectMergeRequests(_ context.Context, projectID int) (review.ProjectMergeRequests, error) {
	f.lastID = projectID
	return f.scoped, f.err
}

type fakeGitHub struct {
	projects []review.Project
	reviews  []review.ReviewSummary
	last     review.Project
	err      error
}

func (f *fakeGitHub) ListProjects(context.Context, string) ([]review.Project, error) {
	return f.projects, f.err
}
func (f *fakeGitHub) ListProjectReviews(_ context.Context, project review.Project, _ string) ([]review.ReviewSummary, error) {
	f.last = project
	return f.reviews, f.err
}

func sampleFetch() review.FetchResult {
	return review.FetchResult{
		DiffFiles: []review.DiffFile{{
			OldPath: "main.py", NewPath: "main.py",
			Diff: "@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
		}},
		Metadata: review.Metadata{
			Title: "Test PR", Description: "A test", SourceBranch: "feature",
			TargetBranch: "main", WebURL: "https://github.com/o/r/pull/1",
		},
	}
}

func testServer(t *testing.T, plat *fakePlatform, prov *fakeProvider) *Server {
	t.Helper()
	if plat == nil {
		plat = &fakePlatform{fetch: sampleFetch(), files: map[string]string{"main.py": "import os\nimport sys\n"}}
	}
	if prov == nil {
		prov = &fakeProvider{result: review.Result{
			Summary:  "Looks good.",
			Comments: []review.Comment{{File: "main.py", Line: 2, Body: "Consider a constant.", Severity: "info"}},
		}}
	}
	gl := &fakeGitLab{
		projects: []review.ProjectSummary{{ProjectID: 7, ProjectPath: "group/app", WebURL: "https://gitlab.com/group/app"}},
		grouped: []review.ProjectMergeRequests{{
			ProjectID: 7, ProjectPath: "group/app",
			MergeRequests: []review.MergeRequestSummary{{ProjectID: 7, ProjectPath: "group/app", IID: 3, Title: "Login"}},
		}},
		scoped: review.ProjectMergeRequests{
			ProjectID: 7, ProjectPath: "group/app",
			MergeRequests: []review.MergeRequestSummary{{ProjectID: 7, ProjectPath: "group/app", IID: 3, Title: "Login"}},
		},
	}
	gh := &fakeGitHub{
		projects: []review.Project{{Platform: "github", ID: "owner/repo", Path: "owner/repo", WebURL: "https://github.com/owner/repo"}},
		reviews:  []review.ReviewSummary{{Project: review.Project{ID: "owner/repo", Path: "owner/repo", WebURL: "https://github.com/owner/repo"}, Number: 9, Title: "catalog"}},
	}
	s := &Server{
		Settings: config.Settings{
			Provider: "anthropic", Model: "claude-test", Focus: []string{"bugs"},
			MaxComments: 10, ParallelThreshold: 10, GitLabURL: "https://gitlab.com", GitHubAPI: "https://api.github.com",
		},
		Jobs: newJobStore(),
		Now:  func() time.Time { return time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC) },
		NewID: func() string {
			return "fixed-id"
		},
		NewPlatform: func(review.Info) (review.Platform, error) { return plat, nil },
		NewProvider: func(string, string) (review.Provider, error) { return prov, nil },
		GitLab:      func() (gitlabSurface, error) { return gl, nil },
		GitHub:      func() (platform.Catalog, error) { return gh, nil },
		Discover: func(context.Context) []provider.Models {
			return []provider.Models{
				{Provider: "anthropic", Models: []string{"claude-test"}, Available: true},
				{Provider: "google", Models: []string{}, Available: false, Error: "GEMINI_API_KEY is not set"},
			}
		},
	}
	return s
}

func doJSON(t *testing.T, h http.Handler, method, path, body string, hdr map[string]string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func waitJob(t *testing.T, h http.Handler, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp := doJSON(t, h, http.MethodGet, "/api/reviews/"+id, "", nil)
		data := decode(t, resp)
		status, _ := data["status"].(string)
		if status == "complete" || status == "posted" || status == "failed" {
			return data
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return nil
}

func TestHealth(t *testing.T) {
	resp := doJSON(t, testServer(t, nil, nil).Handler(), http.MethodGet, "/api/health", "", nil)
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	if decode(t, resp)["status"] != "ok" {
		t.Fatal("health")
	}
}

func TestConfigDefaultsOmitsSecrets(t *testing.T) {
	s := testServer(t, nil, nil)
	s.Settings.Model = "claude-sonnet-4-20250514"
	resp := doJSON(t, s.Handler(), http.MethodGet, "/api/config/defaults", "", nil)
	data := decode(t, resp)
	raw, _ := io.ReadAll(resp.Body)
	_ = raw
	if data["provider"] != "anthropic" || data["model"] != "claude-sonnet-4-20250514" || data["max_comments"] != float64(10) {
		t.Fatalf("%v", data)
	}
	encoded, _ := json.Marshal(data)
	if strings.Contains(string(encoded), "sk-") || strings.Contains(string(encoded), "TOKEN") || strings.Contains(string(encoded), "apiKey") {
		t.Fatalf("secret in %s", encoded)
	}
}

func TestSubmitRequiresModel(t *testing.T) {
	resp := doJSON(t, testServer(t, nil, nil).Handler(), http.MethodPost, "/api/reviews", `{"url":"https://github.com/o/r/pull/1"}`, nil)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestSubmitAndResults(t *testing.T) {
	ids := 0
	s := testServer(t, nil, nil)
	s.NewID = func() string {
		ids++
		if ids == 1 {
			return "job-1"
		}
		return "c1"
	}
	h := s.Handler()
	resp := doJSON(t, h, http.MethodPost, "/api/reviews", `{"url":"https://github.com/o/r/pull/1","model":"test-model"}`, nil)
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d %v", resp.StatusCode, decode(t, resp))
	}
	data := decode(t, resp)
	if data["job_id"] != "job-1" || data["status"] != "pending" {
		t.Fatalf("%v", data)
	}
	waitJob(t, h, "job-1")
	resp = doJSON(t, h, http.MethodGet, "/api/reviews/job-1/results", "", nil)
	got := decode(t, resp)
	if got["summary"] != "Looks good." {
		t.Fatalf("%v", got)
	}
	comments := got["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("%v", comments)
	}
	c0 := comments[0].(map[string]any)
	if c0["file"] != "main.py" || c0["line"] != float64(2) {
		t.Fatalf("%v", c0)
	}
	ctx := c0["diff_context"].([]any)
	if len(ctx) == 0 || !strings.Contains(ctx[0].(string), "import sys") {
		t.Fatalf("diff_context = %v", ctx)
	}
}

func TestResultsGuards(t *testing.T) {
	s := testServer(t, nil, nil)
	h := s.Handler()
	resp := doJSON(t, h, http.MethodGet, "/api/reviews/missing/results", "", nil)
	if resp.StatusCode != 404 {
		t.Fatal(resp.Status)
	}
	s.Jobs.create(&job{ID: "pending", URL: "https://github.com/o/r/pull/1", Status: "pending", CreatedAt: s.Now()})
	resp = doJSON(t, h, http.MethodGet, "/api/reviews/pending/results", "", nil)
	if resp.StatusCode != 409 {
		t.Fatal(resp.Status)
	}
	s.Jobs.create(&job{ID: "failed", URL: "u", Status: "failed", Error: "Provider timeout", CreatedAt: s.Now()})
	resp = doJSON(t, h, http.MethodGet, "/api/reviews/failed/results", "", nil)
	if resp.StatusCode != 500 || !strings.Contains(decode(t, resp)["detail"].(string), "Provider timeout") {
		t.Fatal(resp.Status)
	}
}

func TestEditComment(t *testing.T) {
	s := testServer(t, nil, nil)
	s.Jobs.create(&job{
		ID: "edit", URL: "u", Status: "complete", CreatedAt: s.Now(),
		Comments: []commentDetail{{ID: "c1", File: "main.py", Line: 10, Body: "Original body", Severity: "warning", IsNewLine: true, DiffContext: []string{}}},
	})
	h := s.Handler()
	resp := doJSON(t, h, http.MethodPatch, "/api/reviews/edit/comments/c1", `{"body":"Edited body"}`, nil)
	if resp.StatusCode != 200 || decode(t, resp)["body"] != "Edited body" {
		t.Fatal(resp.Status)
	}
	if s.Jobs.get("edit").Comments[0].Body != "Edited body" {
		t.Fatal("not persisted")
	}
	resp = doJSON(t, h, http.MethodPatch, "/api/reviews/edit/comments/nope", `{"body":"x"}`, nil)
	if resp.StatusCode != 404 {
		t.Fatal(resp.Status)
	}
}

func TestPostApprovedCommentsAndGuards(t *testing.T) {
	plat := &fakePlatform{fetch: sampleFetch()}
	s := testServer(t, plat, nil)
	s.Jobs.create(&job{
		ID: "post", URL: "https://github.com/o/r/pull/1", Status: "complete", CreatedAt: s.Now(),
		Platform: plat, Info: review.Info{Platform: "github", Namespace: "o", Project: "r", IID: 1},
		Comments: []commentDetail{
			{ID: "c1", File: "a.py", Line: 1, Body: "Fix this", Severity: "error", IsNewLine: true},
			{ID: "c2", File: "b.py", Line: 5, Body: "Consider this", Severity: "info", IsNewLine: true},
		},
	})
	h := s.Handler()
	resp := doJSON(t, h, http.MethodPost, "/api/reviews/post/post", `{"comment_ids":["c1"],"summary":"Edited summary"}`, nil)
	if resp.StatusCode != 200 || decode(t, resp)["status"] != "posted" {
		t.Fatal(resp.Status)
	}
	if plat.posted == nil || plat.posted.Summary != "Edited summary" || len(plat.posted.Comments) != 1 || plat.posted.Comments[0].File != "a.py" {
		t.Fatalf("posted = %+v", plat.posted)
	}
	resp = doJSON(t, h, http.MethodPost, "/api/reviews/post/post", `{"comment_ids":["c1"],"summary":"x"}`, nil)
	if resp.StatusCode != 409 {
		t.Fatalf("double post status = %d", resp.StatusCode)
	}
	s.Jobs.create(&job{ID: "early", URL: "u", Status: "pending", CreatedAt: s.Now()})
	resp = doJSON(t, h, http.MethodPost, "/api/reviews/early/post", `{"comment_ids":[],"summary":"s"}`, nil)
	if resp.StatusCode != 409 {
		t.Fatal(resp.Status)
	}
}

func TestInvalidURLFailsJob(t *testing.T) {
	s := testServer(t, nil, nil)
	s.NewPlatform = func(info review.Info) (review.Platform, error) {
		return nil, errors.New("should not build platform")
	}
	h := s.Handler()
	resp := doJSON(t, h, http.MethodPost, "/api/reviews", `{"url":"https://example.com/not-a-pr","model":"m"}`, nil)
	id := decode(t, resp)["job_id"].(string)
	got := waitJob(t, h, id)
	if got["status"] != "failed" || got["error_type"] != "invalid_url" {
		t.Fatalf("%v", got)
	}
}

func TestProviderModelsNoSecrets(t *testing.T) {
	resp := doJSON(t, testServer(t, nil, nil).Handler(), http.MethodGet, "/api/providers/models", "", nil)
	data := decode(t, resp)
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), "secret-key") || strings.Contains(string(raw), "sk-") {
		t.Fatalf("%s", raw)
	}
	providers := data["providers"].([]any)
	if len(providers) != 2 {
		t.Fatalf("%v", providers)
	}
}

func TestDiscoverOneUsesLiveOpenAICatalog(t *testing.T) {
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("openai", auth.Credential{Type: auth.TypeAPIKey, APIKey: "test-key"}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		status    int
		available bool
		models    int
	}{
		{name: "success", status: http.StatusOK, available: true, models: 1},
		{name: "failure", status: http.StatusBadGateway, available: false, models: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing API key")
				}
				w.WriteHeader(tc.status)
				if tc.status == http.StatusOK {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-live"}}})
				}
			}))
			defer srv.Close()

			settings := config.Settings{Providers: config.ProvidersFile{Endpoints: map[string]config.ProviderEndpoint{
				"openai": {BaseURL: srv.URL},
			}}}
			got := discoverOne(context.Background(), settings, store, srv.Client(), "openai")
			if got.Available != tc.available || len(got.Models) != tc.models {
				t.Fatalf("%+v", got)
			}
		})
	}
}

func TestDiscoverOneOpenAIOAuthUsesExchangedAPIKey(t *testing.T) {
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("openai", auth.Credential{Type: auth.TypeOAuth, Access: "chatgpt-oauth-token", APIKey: "exchanged-key"}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer exchanged-key" {
			t.Fatalf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]string{{"id": "gpt-live"}}})
	}))
	defer srv.Close()

	settings := config.Settings{Providers: config.ProvidersFile{Endpoints: map[string]config.ProviderEndpoint{
		"openai": {BaseURL: srv.URL},
	}}}
	got := discoverOne(context.Background(), settings, store, srv.Client(), "openai")
	if !got.Available || len(got.Models) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestDiscoverOneOpenAIOAuthWithoutAPIKeyDoesNotCallCatalog(t *testing.T) {
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("openai", auth.Credential{Type: auth.TypeOAuth, Access: "chatgpt-oauth-token"}); err != nil {
		t.Fatal(err)
	}
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		t.Fatalf("catalog request used OAuth token: %q", r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	settings := config.Settings{Providers: config.ProvidersFile{Endpoints: map[string]config.ProviderEndpoint{
		"openai": {BaseURL: srv.URL},
	}}}
	got := discoverOne(context.Background(), settings, store, srv.Client(), "openai")
	if called || got.Available || !strings.Contains(got.Error, "run `mr-reviewer auth login openai` again") {
		t.Fatalf("called=%v catalog=%+v", called, got)
	}
}

func TestGitLabCatalog(t *testing.T) {
	h := testServer(t, nil, nil).Handler()
	resp := doJSON(t, h, http.MethodGet, "/api/gitlab/projects", "", nil)
	data := decode(t, resp)
	projects := data["projects"].([]any)
	if len(projects) != 1 || projects[0].(map[string]any)["project_id"] != float64(7) {
		t.Fatalf("%v", data)
	}
	resp = doJSON(t, h, http.MethodGet, "/api/gitlab/projects/7/merge-requests", "", nil)
	if decode(t, resp)["project_id"] != float64(7) {
		t.Fatal("scoped")
	}
}

func TestGitHubCatalogScoping(t *testing.T) {
	gh := &fakeGitHub{
		projects: []review.Project{{Platform: "github", ID: "owner/repo", Path: "owner/repo"}},
		reviews:  []review.ReviewSummary{{Number: 4, Title: "only this repo"}},
	}
	s := testServer(t, nil, nil)
	s.GitHub = func() (platform.Catalog, error) { return gh, nil }
	h := s.Handler()
	resp := doJSON(t, h, http.MethodGet, "/api/github/projects", "", nil)
	if len(decode(t, resp)["projects"].([]any)) != 1 {
		t.Fatal("projects")
	}
	resp = doJSON(t, h, http.MethodGet, "/api/github/projects/owner/repo/pull-requests", "", nil)
	data := decode(t, resp)
	if data["id"] != "owner/repo" {
		t.Fatalf("%v", data)
	}
	if gh.last.ID != "owner/repo" {
		t.Fatalf("scoped to %q", gh.last.ID)
	}
}

func TestAuthFailures(t *testing.T) {
	s := testServer(t, nil, nil)
	s.Token = "test-token"
	h := s.Handler()
	if doJSON(t, h, http.MethodGet, "/api/health", "", nil).StatusCode != 200 {
		t.Fatal("health should skip auth")
	}
	if doJSON(t, h, http.MethodGet, "/api/config/defaults", "", nil).StatusCode != 403 {
		t.Fatal("missing token")
	}
	if doJSON(t, h, http.MethodGet, "/api/config/defaults", "", map[string]string{"Authorization": "Bearer wrong-token"}).StatusCode != 403 {
		t.Fatal("wrong token")
	}
	resp := doJSON(t, h, http.MethodGet, "/api/config/defaults", "", map[string]string{"Authorization": "Bearer test-token"})
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	req := httptest.NewRequest(http.MethodOptions, "/api/config/defaults", nil)
	req.Header.Set("Origin", "null")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || rec.Header().Get("Access-Control-Allow-Origin") != "null" {
		t.Fatalf("preflight %d %v", rec.Code, rec.Header())
	}
}

func TestNewUsesConfigAndAuthStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MR_REVIEWER_HOME", dir)
	t.Setenv("MR_REVIEWER_CONFIG", "")
	t.Setenv("MR_REVIEWER_PROVIDERS", "")
	t.Setenv("MR_REVIEWER_PROVIDER", "echo")
	t.Setenv("ANTHROPIC_API_KEY", "")
	st, err := auth.OpenStore(filepath.Join(dir, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := New(config.Load(), st, "")
	resp := doJSON(t, s.Handler(), http.MethodGet, "/api/config/defaults", "", nil)
	data := decode(t, resp)
	if data["provider"] != "echo" {
		t.Fatalf("%v", data)
	}
	resp = doJSON(t, s.Handler(), http.MethodGet, "/api/providers/models", "", nil)
	raw, _ := json.Marshal(decode(t, resp))
	if strings.Contains(string(raw), "ANTHROPIC_API_KEY=sk") {
		t.Fatalf("secret %s", raw)
	}
}

func TestGitLabInsecureRejected(t *testing.T) {
	s := testServer(t, nil, nil)
	s.Settings.GitLabURL = "http://gitlab.local"
	s.Settings.AllowInsecureGitLab = false
	resp := doJSON(t, s.Handler(), http.MethodGet, "/api/gitlab/projects", "", nil)
	if resp.StatusCode != 400 {
		t.Fatal(resp.Status)
	}
}

func TestOnboardingStatusAndComplete(t *testing.T) {
	dir := t.TempDir()
	store, err := auth.OpenStore(filepath.Join(dir, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved config.OnboardingState
	s := testServer(t, nil, nil)
	s.Store = store
	s.sessions = newAuthSessionStore()
	s.Settings.GitHubAPI = "https://api.github.com"
	s.SaveOnboarding = func(state config.OnboardingState) error { saved = state; return nil }

	resp := doJSON(t, s.Handler(), http.MethodGet, "/api/onboarding", "", nil)
	status := decode(t, resp)
	if status["complete"] != false {
		t.Fatalf("expected incomplete: %v", status)
	}

	resp = doJSON(t, s.Handler(), http.MethodPost, "/api/onboarding/secret", `{"kind":"provider","name":"anthropic","secret":"provider-secret"}`, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("provider secret: %s", decode(t, resp))
	}
	resp = doJSON(t, s.Handler(), http.MethodPost, "/api/onboarding/secret", `{"kind":"platform","name":"github","secret":"platform-secret"}`, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("platform secret: %s", decode(t, resp))
	}
	resp = doJSON(t, s.Handler(), http.MethodPost, "/api/onboarding", `{"provider":"anthropic","platform":"github"}`, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("complete: %s", decode(t, resp))
	}
	if saved.Provider != "anthropic" || saved.Platform != "github" || saved.ProviderFingerprint == "" {
		t.Fatalf("saved = %+v", saved)
	}
	if decode(t, resp)["complete"] != true {
		t.Fatalf("status after complete: %v", decode(t, doJSON(t, s.Handler(), http.MethodGet, "/api/onboarding", "", nil)))
	}
}

func TestAuthDeviceSessionBlocksUntilCompleteOrCancel(t *testing.T) {
	s := testServer(t, nil, nil)
	s.sessions = newAuthSessionStore()
	s.DeviceFlow = func(name string) (auth.DeviceConfig, error) {
		if name != "github" {
			t.Fatalf("name = %s", name)
		}
		return auth.DeviceConfig{}, errors.New("authorization pending in test")
	}
	resp := doJSON(t, s.Handler(), http.MethodPost, "/api/auth/sessions", `{"kind":"platform","name":"github","method":"device"}`, nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("start: %d %v", resp.StatusCode, decode(t, resp))
	}
	started := decode(t, resp)
	if started["status"] != "pending" && started["status"] != "failed" {
		t.Fatalf("session = %v", started)
	}
	id, _ := started["session_id"].(string)
	got := decode(t, doJSON(t, s.Handler(), http.MethodGet, "/api/auth/sessions/"+id, "", nil))
	errText, _ := got["error"].(string)
	if got["status"] != "failed" || !strings.Contains(errText, "authorization pending") {
		t.Fatalf("status = %v", got)
	}
	cancel := decode(t, doJSON(t, s.Handler(), http.MethodPost, "/api/auth/sessions/"+id+"/cancel", "", nil))
	if cancel["session_id"] != id {
		t.Fatalf("cancel = %v", cancel)
	}
}
