package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

// GitLab talks to the GitLab REST API (gitlab.com or self-hosted).
type GitLab struct {
	BaseURL     string
	Credentials func(context.Context) (auth.PlatformCredential, error)
	HTTP        *http.Client
	diffRefs    *diffRefs
	diffFiles   []review.DiffFile
}

type diffRefs struct {
	BaseSHA  string
	StartSHA string
	HeadSHA  string
}

func (c *GitLab) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *GitLab) origin() string {
	b := strings.TrimRight(c.BaseURL, "/")
	if b == "" {
		b = "https://gitlab.com"
	}
	return b
}

func (c *GitLab) api() string {
	return c.origin() + "/api/v4"
}

func (c *GitLab) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.api()+path, rdr)
	if err != nil {
		return nil, err
	}
	if err := c.applyAuth(ctx, req); err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.client().Do(req)
}

func encodeProject(namespace, project string) string {
	return url.PathEscape(namespace + "/" + project)
}

func (c *GitLab) decodeAll(resp *http.Response, dest any) error {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gitlab API %s: %s", resp.Status, b)
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(b, dest)
}

// FetchChanges implements review.Platform.
func (c *GitLab) FetchChanges(ctx context.Context, info review.Info) (review.FetchResult, error) {
	pid := encodeProject(info.Namespace, info.Project)
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/merge_requests/%d/changes", pid, info.IID), nil)
	if err != nil {
		return review.FetchResult{}, err
	}
	var payload struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		SourceBranch string `json:"source_branch"`
		TargetBranch string `json:"target_branch"`
		WebURL       string `json:"web_url"`
		DiffRefs     struct {
			BaseSHA  string `json:"base_sha"`
			StartSHA string `json:"start_sha"`
			HeadSHA  string `json:"head_sha"`
		} `json:"diff_refs"`
		Changes []struct {
			OldPath     string `json:"old_path"`
			NewPath     string `json:"new_path"`
			Diff        string `json:"diff"`
			NewFile     bool   `json:"new_file"`
			RenamedFile bool   `json:"renamed_file"`
			DeletedFile bool   `json:"deleted_file"`
		} `json:"changes"`
	}
	if err := c.decodeAll(resp, &payload); err != nil {
		return review.FetchResult{}, fmt.Errorf("gitlab fetch changes: %w", err)
	}
	c.diffRefs = &diffRefs{BaseSHA: payload.DiffRefs.BaseSHA, StartSHA: payload.DiffRefs.StartSHA, HeadSHA: payload.DiffRefs.HeadSHA}
	var files []review.DiffFile
	for _, ch := range payload.Changes {
		files = append(files, review.DiffFile{
			OldPath:     ch.OldPath,
			NewPath:     ch.NewPath,
			Diff:        ch.Diff,
			NewFile:     ch.NewFile,
			RenamedFile: ch.RenamedFile,
			DeletedFile: ch.DeletedFile,
		})
	}
	c.diffFiles = files
	return review.FetchResult{
		DiffFiles: files,
		Metadata: review.Metadata{
			Title:        payload.Title,
			Description:  payload.Description,
			SourceBranch: payload.SourceBranch,
			TargetBranch: payload.TargetBranch,
			WebURL:       payload.WebURL,
		},
	}, nil
}

// FetchFile implements review.Platform.
func (c *GitLab) FetchFile(ctx context.Context, info review.Info, path, ref string) (string, bool, error) {
	pid := encodeProject(info.Namespace, info.Project)
	p := fmt.Sprintf("/projects/%s/repository/files/%s/raw?ref=%s", pid, url.PathEscape(path), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.api()+p, nil)
	if err != nil {
		return "", false, err
	}
	if err := c.applyAuth(ctx, req); err != nil {
		return "", false, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, nil
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false, err
	}
	return string(b), true, nil
}

func (c *GitLab) applyAuth(ctx context.Context, req *http.Request) error {
	if c.Credentials == nil {
		return nil
	}
	credential, err := c.Credentials(ctx)
	if err != nil {
		return err
	}
	return auth.ApplyPlatformAuth(req.Header, "gitlab", credential)
}

// PostReview implements review.Platform.
func (c *GitLab) PostReview(ctx context.Context, info review.Info, result review.Result, _ []review.DiffLine) error {
	if c.diffRefs == nil {
		return fmt.Errorf("cannot post review: FetchChanges() must be called first to cache diff refs")
	}
	pid := encodeProject(info.Namespace, info.Project)
	for _, cm := range result.Comments {
		oldPath, newPath := c.findPaths(cm.File)
		pos := map[string]any{
			"position_type": "text",
			"base_sha":      c.diffRefs.BaseSHA,
			"start_sha":     c.diffRefs.StartSHA,
			"head_sha":      c.diffRefs.HeadSHA,
			"old_path":      oldPath,
			"new_path":      newPath,
		}
		if cm.IsNewLine {
			pos["new_line"] = cm.Line
		} else {
			pos["old_line"] = cm.Line
		}
		resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", pid, info.IID), map[string]any{
			"body":     cm.Body,
			"position": pos,
		})
		if err != nil {
			return err
		}
		_ = c.decodeAll(resp, nil)
	}
	summary := "## AI Code Review\n\n" + result.Summary
	if len(result.Comments) > 0 {
		var e, w, i int
		for _, cm := range result.Comments {
			switch cm.Severity {
			case "error":
				e++
			case "warning":
				w++
			default:
				i++
			}
		}
		summary += fmt.Sprintf("\n\n**%d inline comments posted:** %d errors, %d warnings, %d suggestions", len(result.Comments), e, w, i)
	}
	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/merge_requests/%d/notes", pid, info.IID), map[string]any{"body": summary})
	if err != nil {
		return err
	}
	return c.decodeAll(resp, nil)
}

func (c *GitLab) findPaths(file string) (string, string) {
	for _, df := range c.diffFiles {
		if df.NewPath == file || df.OldPath == file {
			return df.OldPath, df.NewPath
		}
	}
	return file, file
}

// ListVisibleMergeRequests lists open MRs the token can see, grouped by project.
func (c *GitLab) ListVisibleMergeRequests(ctx context.Context, search string) ([]review.ProjectMergeRequests, error) {
	query := strings.ToLower(strings.TrimSpace(search))
	var raw []gitlabMR
	page := 1
	for {
		resp, err := c.do(ctx, http.MethodGet, "/merge_requests?scope=all&state=opened&order_by=updated_at&sort=desc&per_page=100&page="+strconv.Itoa(page), nil)
		if err != nil {
			return nil, err
		}
		var batch []gitlabMR
		if err := c.decodeAll(resp, &batch); err != nil {
			return nil, fmt.Errorf("gitlab list merge requests: %w", err)
		}
		raw = append(raw, batch...)
		if len(batch) < 100 {
			break
		}
		page++
		if page > 20 {
			break
		}
	}
	groups := map[int]*review.ProjectMergeRequests{}
	var order []int
	for _, mr := range raw {
		path := mr.projectPath()
		title := mr.Title
		if query != "" && !strings.Contains(strings.ToLower(path), query) && !strings.Contains(strings.ToLower(title), query) {
			continue
		}
		g, ok := groups[mr.ProjectID]
		if !ok {
			g = &review.ProjectMergeRequests{ProjectID: mr.ProjectID, ProjectPath: path}
			groups[mr.ProjectID] = g
			order = append(order, mr.ProjectID)
		}
		g.MergeRequests = append(g.MergeRequests, review.MergeRequestSummary{
			ProjectID:    mr.ProjectID,
			ProjectPath:  path,
			IID:          mr.IID,
			Title:        title,
			Author:       mr.Author.Name,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			UpdatedAt:    mr.UpdatedAt,
			WebURL:       mr.WebURL,
			Draft:        mr.Draft,
		})
	}
	out := make([]review.ProjectMergeRequests, 0, len(order))
	for _, id := range order {
		out = append(out, *groups[id])
	}
	return out, nil
}

// ListVisibleProjects lists membership projects.
func (c *GitLab) ListVisibleProjects(ctx context.Context, search string) ([]review.ProjectSummary, error) {
	query := strings.ToLower(strings.TrimSpace(search))
	var raw []struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
	}
	page := 1
	for {
		resp, err := c.do(ctx, http.MethodGet, "/projects?membership=true&archived=false&simple=true&order_by=path&sort=asc&per_page=100&page="+strconv.Itoa(page), nil)
		if err != nil {
			return nil, err
		}
		var batch []struct {
			ID                int    `json:"id"`
			PathWithNamespace string `json:"path_with_namespace"`
			WebURL            string `json:"web_url"`
		}
		if err := c.decodeAll(resp, &batch); err != nil {
			return nil, fmt.Errorf("gitlab list projects: %w", err)
		}
		raw = append(raw, batch...)
		if len(batch) < 100 {
			break
		}
		page++
		if page > 20 {
			break
		}
	}
	var out []review.ProjectSummary
	for _, p := range raw {
		if query != "" && !strings.Contains(strings.ToLower(p.PathWithNamespace), query) {
			continue
		}
		out = append(out, review.ProjectSummary{ProjectID: p.ID, ProjectPath: p.PathWithNamespace, WebURL: p.WebURL})
	}
	return out, nil
}

// ListProjectMergeRequests lists open MRs for one project.
func (c *GitLab) ListProjectMergeRequests(ctx context.Context, projectID int) (review.ProjectMergeRequests, error) {
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%d", projectID), nil)
	if err != nil {
		return review.ProjectMergeRequests{}, err
	}
	var proj struct {
		ID                int    `json:"id"`
		PathWithNamespace string `json:"path_with_namespace"`
	}
	if err := c.decodeAll(resp, &proj); err != nil {
		return review.ProjectMergeRequests{}, fmt.Errorf("gitlab project: %w", err)
	}
	resp, err = c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%d/merge_requests?state=opened&order_by=updated_at&sort=desc&per_page=100", projectID), nil)
	if err != nil {
		return review.ProjectMergeRequests{}, err
	}
	var mrs []gitlabMR
	if err := c.decodeAll(resp, &mrs); err != nil {
		return review.ProjectMergeRequests{}, fmt.Errorf("gitlab project MRs: %w", err)
	}
	out := review.ProjectMergeRequests{ProjectID: proj.ID, ProjectPath: proj.PathWithNamespace}
	for _, mr := range mrs {
		out.MergeRequests = append(out.MergeRequests, review.MergeRequestSummary{
			ProjectID:    proj.ID,
			ProjectPath:  proj.PathWithNamespace,
			IID:          mr.IID,
			Title:        mr.Title,
			Author:       mr.Author.Name,
			SourceBranch: mr.SourceBranch,
			TargetBranch: mr.TargetBranch,
			UpdatedAt:    mr.UpdatedAt,
			WebURL:       mr.WebURL,
			Draft:        mr.Draft,
		})
	}
	return out, nil
}

type gitlabMR struct {
	ProjectID    int    `json:"project_id"`
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	UpdatedAt    string `json:"updated_at"`
	WebURL       string `json:"web_url"`
	Draft        bool   `json:"draft"`
	Author       struct {
		Name string `json:"name"`
	} `json:"author"`
	References struct {
		Full string `json:"full"`
	} `json:"references"`
}

func (m gitlabMR) projectPath() string {
	if m.References.Full != "" {
		if i := strings.LastIndex(m.References.Full, "!"); i >= 0 {
			return m.References.Full[:i]
		}
	}
	return strconv.Itoa(m.ProjectID)
}

// Browser is the GitLab dashboard surface.
type Browser interface {
	ListVisibleMergeRequests(ctx context.Context, search string) ([]review.ProjectMergeRequests, error)
	ListVisibleProjects(ctx context.Context, search string) ([]review.ProjectSummary, error)
	ListProjectMergeRequests(ctx context.Context, projectID int) (review.ProjectMergeRequests, error)
}
