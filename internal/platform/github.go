package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

const maxFiles = 3000

// GitHub talks to the GitHub REST API.
type GitHub struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
	headSHA string
}

func (c *GitHub) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (c *GitHub) base() string {
	b := strings.TrimRight(c.BaseURL, "/")
	if b == "" {
		b = "https://api.github.com"
	}
	return b
}

func (c *GitHub) do(ctx context.Context, method, path string, body any, accept string) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rdr)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	} else {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.client().Do(req)
}

// FetchChanges implements review.Platform.
func (c *GitHub) FetchChanges(ctx context.Context, info review.Info) (review.FetchResult, error) {
	owner, repo, n := info.Namespace, info.Project, info.IID
	resp, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, n), nil, "")
	if err != nil {
		return review.FetchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return review.FetchResult{}, fmt.Errorf("failed to fetch PR #%d: %d %s", n, resp.StatusCode, b)
	}
	var pr struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		HTML  string `json:"html_url"`
		Head  struct {
			SHA string `json:"sha"`
			Ref string `json:"ref"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return review.FetchResult{}, err
	}
	c.headSHA = pr.Head.SHA

	var files []map[string]any
	page := 1
	for len(files) < maxFiles {
		fr, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", owner, repo, n, page), nil, "")
		if err != nil {
			return review.FetchResult{}, err
		}
		var batch []map[string]any
		if err := json.NewDecoder(fr.Body).Decode(&batch); err != nil {
			fr.Body.Close()
			return review.FetchResult{}, err
		}
		fr.Body.Close()
		if fr.StatusCode != http.StatusOK {
			return review.FetchResult{}, fmt.Errorf("failed to fetch PR files: %d", fr.StatusCode)
		}
		if len(batch) == 0 {
			break
		}
		files = append(files, batch...)
		if len(batch) < 100 {
			break
		}
		page++
	}

	var diffs []review.DiffFile
	for _, f := range files {
		filename, _ := f["filename"].(string)
		prev, _ := f["previous_filename"].(string)
		if prev == "" {
			prev = filename
		}
		patch, _ := f["patch"].(string)
		status, _ := f["status"].(string)
		diffs = append(diffs, review.DiffFile{
			OldPath:     prev,
			NewPath:     filename,
			Diff:        patch,
			NewFile:     status == "added",
			RenamedFile: status == "renamed",
			DeletedFile: status == "removed",
		})
	}
	return review.FetchResult{
		DiffFiles: diffs,
		Metadata: review.Metadata{
			Title:        pr.Title,
			Description:  pr.Body,
			SourceBranch: pr.Head.Ref,
			TargetBranch: pr.Base.Ref,
			WebURL:       pr.HTML,
		},
	}, nil
}

// FetchFile implements review.Platform.
func (c *GitHub) FetchFile(ctx context.Context, info review.Info, path, ref string) (string, bool, error) {
	p := fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", info.Namespace, info.Project, path, url.QueryEscape(ref))
	resp, err := c.do(ctx, http.MethodGet, p, nil, "application/vnd.github.raw+json")
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

// PostReview implements review.Platform.
func (c *GitHub) PostReview(ctx context.Context, info review.Info, result review.Result, _ []review.DiffLine) error {
	if c.headSHA == "" {
		return fmt.Errorf("cannot post review: FetchChanges() must be called first to cache head SHA")
	}
	var comments []map[string]any
	for _, cm := range result.Comments {
		side := "RIGHT"
		if !cm.IsNewLine {
			side = "LEFT"
		}
		comments = append(comments, map[string]any{
			"path": cm.File,
			"line": cm.Line,
			"body": cm.Body,
			"side": side,
		})
	}
	payload := map[string]any{
		"commit_id": c.headSHA,
		"body":      result.Summary,
		"event":     "COMMENT",
		"comments":  comments,
	}
	resp, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", info.Namespace, info.Project, info.IID), payload, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to post review: %d %s", resp.StatusCode, b)
	}
	return nil
}
