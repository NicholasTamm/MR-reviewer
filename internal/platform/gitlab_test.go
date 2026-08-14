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

func TestGitLabFetchPostAndDashboard(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if ep := r.URL.EscapedPath(); strings.Contains(ep, "group%2Fproject") {
			path = strings.ReplaceAll(ep, "group%2Fproject", "group/project")
		}
		switch {
		case path == "/api/v4/projects/group/project/merge_requests/7/changes":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"title":         "Fix",
				"description":   "d",
				"source_branch": "feat",
				"target_branch": "main",
				"web_url":       "https://gitlab.com/group/project/-/merge_requests/7",
				"diff_refs":     map[string]string{"base_sha": "b", "start_sha": "s", "head_sha": "h"},
				"changes": []map[string]any{{
					"old_path": "a.py", "new_path": "a.py",
					"diff": "@@ -1,1 +1,2 @@\n x\n+y\n",
				}},
			})
		case strings.Contains(path, "/repository/files/"):
			_, _ = w.Write([]byte("x\ny\n"))
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/discussions"):
			posts++
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "new_line") {
				t.Errorf("discussion body = %s", body)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/notes"):
			posts++
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 2})
		case path == "/api/v4/merge_requests":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"project_id": 9, "iid": 7, "title": "Fix login",
				"source_branch": "feat", "target_branch": "main",
				"updated_at": "2024-01-01", "web_url": "https://gitlab.com/group/project/-/merge_requests/7",
				"draft": false, "author": map[string]string{"name": "Ada"},
				"references": map[string]string{"full": "group/project!7"},
			}})
		case path == "/api/v4/projects" && r.URL.Query().Get("membership") == "true":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": 9, "path_with_namespace": "group/project", "web_url": "https://gitlab.com/group/project",
			}})
		case path == "/api/v4/projects/9" && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 9, "path_with_namespace": "group/project"})
		case path == "/api/v4/projects/9/merge_requests":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"project_id": 9, "iid": 7, "title": "Fix login",
				"source_branch": "feat", "target_branch": "main",
				"web_url": "https://gitlab.com/group/project/-/merge_requests/7",
				"author":  map[string]string{"name": "Ada"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := &GitLab{BaseURL: srv.URL, Token: "glpat"}
	info := review.Info{Platform: "gitlab", Namespace: "group", Project: "project", IID: 7}
	fr, err := c.FetchChanges(context.Background(), info)
	if err != nil {
		t.Fatal(err)
	}
	if fr.Metadata.Title != "Fix" || len(fr.DiffFiles) != 1 {
		t.Fatalf("%+v", fr)
	}
	body, ok, err := c.FetchFile(context.Background(), info, "a.py", "feat")
	if err != nil || !ok || body == "" {
		t.Fatalf("file ok=%v err=%v body=%q", ok, err, body)
	}
	if err := c.PostReview(context.Background(), info, review.Result{
		Summary:  "ok",
		Comments: []review.Comment{{File: "a.py", Line: 2, Body: "n", Severity: "info", IsNewLine: true}},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if posts < 2 {
		t.Fatalf("posts = %d, want discussion+note", posts)
	}

	groups, err := c.ListVisibleMergeRequests(context.Background(), "login")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].ProjectPath != "group/project" || len(groups[0].MergeRequests) != 1 {
		t.Fatalf("%+v", groups)
	}
	projs, err := c.ListVisibleProjects(context.Background(), "group")
	if err != nil || len(projs) != 1 {
		t.Fatalf("projects = %+v err=%v", projs, err)
	}
	pm, err := c.ListProjectMergeRequests(context.Background(), 9)
	if err != nil || pm.ProjectPath != "group/project" || len(pm.MergeRequests) != 1 {
		t.Fatalf("%+v err=%v", pm, err)
	}
}

func TestGitLabSearchFilters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"project_id": 1, "iid": 1, "title": "Unrelated",
			"references": map[string]string{"full": "other/repo!1"},
		}})
	}))
	defer srv.Close()
	c := &GitLab{BaseURL: srv.URL}
	groups, err := c.ListVisibleMergeRequests(context.Background(), "nomatch")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("%+v", groups)
	}
}
