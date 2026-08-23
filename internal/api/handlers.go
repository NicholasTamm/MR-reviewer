package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	var req reviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeDetail(w, http.StatusUnprocessableEntity, "url is required")
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		writeDetail(w, http.StatusUnprocessableEntity, "model is required")
		return
	}
	if req.Provider == "" {
		req.Provider = s.Settings.Provider
	}
	if len(req.Focus) == 0 {
		req.Focus = append([]string{}, s.Settings.Focus...)
	}
	if req.MaxComments <= 0 {
		req.MaxComments = s.Settings.MaxComments
	}
	j := &job{
		ID:        s.id(),
		URL:       req.URL,
		Status:    "pending",
		Progress:  "Queued",
		CreatedAt: s.now(),
	}
	s.Jobs.create(j)
	created := j.status()
	go s.runReview(j.ID, req)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	j := s.Jobs.get(r.PathValue("job_id"))
	if j == nil {
		writeDetail(w, http.StatusNotFound, "Job not found")
		return
	}
	writeJSON(w, http.StatusOK, j.status())
}

func (s *Server) handleResults(w http.ResponseWriter, r *http.Request) {
	j := s.Jobs.get(r.PathValue("job_id"))
	if j == nil {
		writeDetail(w, http.StatusNotFound, "Job not found")
		return
	}
	if j.Status == "failed" {
		detail := j.Error
		if detail == "" {
			detail = "Review failed"
		}
		writeDetail(w, http.StatusInternalServerError, detail)
		return
	}
	if j.Status != "complete" && j.Status != "posted" {
		writeDetail(w, http.StatusConflict, "Review not ready: "+j.Status)
		return
	}
	if j.Comments == nil {
		j.Comments = []commentDetail{}
	}
	writeJSON(w, http.StatusOK, reviewResponse{
		JobID:    j.ID,
		Summary:  j.Summary,
		Comments: j.Comments,
		Metadata: j.Metadata,
	})
}

func (s *Server) handleEditComment(w http.ResponseWriter, r *http.Request) {
	var req commentEditRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	updated := s.Jobs.editComment(r.PathValue("job_id"), r.PathValue("comment_id"), req.Body)
	if updated == nil {
		if s.Jobs.get(r.PathValue("job_id")) == nil {
			writeDetail(w, http.StatusNotFound, "Job not found")
			return
		}
		writeDetail(w, http.StatusNotFound, "Comment not found")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	var req postRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	jobID := r.PathValue("job_id")
	j := s.Jobs.transition(jobID, "complete", "posting")
	if j == nil {
		stored := s.Jobs.get(jobID)
		if stored == nil {
			writeDetail(w, http.StatusNotFound, "Job not found")
			return
		}
		writeDetail(w, http.StatusConflict, "Cannot post review in state: "+stored.Status)
		return
	}
	if j.Platform == nil || j.Info.Platform == "" {
		s.Jobs.update(jobID, func(cur *job) { cur.Status = "complete" })
		writeDetail(w, http.StatusInternalServerError, "Missing platform client state")
		return
	}
	wanted := map[string]struct{}{}
	for _, id := range req.CommentIDs {
		wanted[id] = struct{}{}
	}
	var comments []review.Comment
	for _, c := range j.Comments {
		if _, ok := wanted[c.ID]; !ok {
			continue
		}
		comments = append(comments, review.Comment{
			File: c.File, Line: c.Line, Body: c.Body, Severity: c.Severity, IsNewLine: c.IsNewLine,
		})
	}
	if comments == nil {
		comments = []review.Comment{}
	}
	err := j.Platform.PostReview(r.Context(), j.Info, review.Result{Summary: req.Summary, Comments: comments}, j.DiffLines)
	if err != nil {
		s.Jobs.update(jobID, func(cur *job) { cur.Status = "complete" })
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	s.Jobs.update(jobID, func(cur *job) {
		cur.Status = "posted"
		cur.Progress = "Posted"
	})
	posted := s.Jobs.get(jobID)
	writeJSON(w, http.StatusOK, posted.status())
}

func (s *Server) handleConfigDefaults(w http.ResponseWriter, _ *http.Request) {
	var model *string
	if strings.TrimSpace(s.Settings.Model) != "" {
		m := s.Settings.Model
		model = &m
	}
	focus := s.Settings.Focus
	if focus == nil {
		focus = []string{}
	}
	writeJSON(w, http.StatusOK, configDefaults{
		Provider:          s.Settings.Provider,
		Model:             model,
		Focus:             focus,
		MaxComments:       s.Settings.MaxComments,
		Parallel:          s.Settings.Parallel,
		ParallelThreshold: s.Settings.ParallelThreshold,
	})
}

func (s *Server) handleProviderModels(w http.ResponseWriter, r *http.Request) {
	var models []providerModelsResponse
	for _, m := range s.Discover(r.Context()) {
		item := providerModelsResponse{
			Provider:  m.Provider,
			Models:    m.Models,
			Available: m.Available,
		}
		if m.Models == nil {
			item.Models = []string{}
		}
		if m.Error != "" {
			err := m.Error
			item.Error = &err
		}
		models = append(models, item)
	}
	if models == nil {
		models = []providerModelsResponse{}
	}
	writeJSON(w, http.StatusOK, providerCatalogResponse{Providers: models})
}

func (s *Server) gitlabClient() (gitlabSurface, error) {
	if !strings.HasPrefix(s.Settings.GitLabURL, "https://") && !s.Settings.AllowInsecureGitLab {
		return nil, errInsecureGitLab
	}
	return s.GitLab()
}

var errInsecureGitLab = fmt.Errorf("GitLab browsing requires an HTTPS MR_REVIEWER_GITLAB_URL")

func (s *Server) handleGitLabMergeRequests(w http.ResponseWriter, r *http.Request) {
	client, err := s.gitlabClient()
	if err != nil {
		if err == errInsecureGitLab {
			writeDetail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	projects, err := client.ListVisibleMergeRequests(r.Context(), r.URL.Query().Get("search"))
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	out := gitlabMRCatalogJSON{Projects: make([]gitlabProjectMRsJSON, 0, len(projects))}
	for _, p := range projects {
		item := gitlabProjectMRsJSON{ProjectID: p.ProjectID, ProjectPath: p.ProjectPath, MergeRequests: []gitlabMRJSON{}}
		for _, mr := range p.MergeRequests {
			item.MergeRequests = append(item.MergeRequests, gitlabMRFrom(mr))
		}
		out.Projects = append(out.Projects, item)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGitLabProjects(w http.ResponseWriter, r *http.Request) {
	client, err := s.gitlabClient()
	if err != nil {
		if err == errInsecureGitLab {
			writeDetail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	projects, err := client.ListVisibleProjects(r.Context(), r.URL.Query().Get("search"))
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	out := gitlabProjectsJSON{Projects: make([]gitlabProjectJSON, 0, len(projects))}
	for _, p := range projects {
		out.Projects = append(out.Projects, gitlabProjectJSON{ProjectID: p.ProjectID, ProjectPath: p.ProjectPath, WebURL: p.WebURL})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGitLabProjectMRs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("project_id"))
	if err != nil || id <= 0 {
		writeDetail(w, http.StatusBadRequest, "GitLab project ID must be a positive integer")
		return
	}
	client, err := s.gitlabClient()
	if err != nil {
		if err == errInsecureGitLab {
			writeDetail(w, http.StatusBadRequest, err.Error())
			return
		}
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	got, err := client.ListProjectMergeRequests(r.Context(), id)
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	out := gitlabProjectMRsJSON{ProjectID: got.ProjectID, ProjectPath: got.ProjectPath, MergeRequests: []gitlabMRJSON{}}
	for _, mr := range got.MergeRequests {
		out.MergeRequests = append(out.MergeRequests, gitlabMRFrom(mr))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGitHubProjects(w http.ResponseWriter, r *http.Request) {
	client, err := s.GitHub()
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	projects, err := client.ListProjects(r.Context(), r.URL.Query().Get("search"))
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	out := githubProjectsJSON{Projects: make([]githubProjectJSON, 0, len(projects))}
	for _, p := range projects {
		out.Projects = append(out.Projects, githubProjectJSON{ID: p.ID, Path: p.Path, WebURL: p.WebURL, Platform: "github"})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGitHubProjectPRs(w http.ResponseWriter, r *http.Request) {
	owner := r.PathValue("owner")
	repo := r.PathValue("repo")
	if owner == "" || repo == "" || strings.Contains(repo, "/") {
		writeDetail(w, http.StatusBadRequest, "github project ID must be owner/repository")
		return
	}
	client, err := s.GitHub()
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	project := review.Project{Platform: "github", ID: owner + "/" + repo, Path: owner + "/" + repo}
	reviews, err := client.ListProjectReviews(r.Context(), project, r.URL.Query().Get("search"))
	if err != nil {
		writeDetail(w, http.StatusBadGateway, err.Error())
		return
	}
	out := githubProjectPRsJSON{ID: project.ID, Path: project.Path, PullRequests: []githubPRJSON{}}
	if len(reviews) > 0 {
		out.WebURL = reviews[0].Project.WebURL
	}
	for _, pr := range reviews {
		out.PullRequests = append(out.PullRequests, githubPRJSON{
			Number: pr.Number, Title: pr.Title, Author: pr.Author,
			SourceBranch: pr.SourceBranch, TargetBranch: pr.TargetBranch,
			UpdatedAt: pr.UpdatedAt, WebURL: pr.WebURL, Draft: pr.Draft,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func gitlabMRFrom(mr review.MergeRequestSummary) gitlabMRJSON {
	return gitlabMRJSON{
		ProjectID: mr.ProjectID, ProjectPath: mr.ProjectPath, IID: mr.IID, Title: mr.Title,
		Author: mr.Author, SourceBranch: mr.SourceBranch, TargetBranch: mr.TargetBranch,
		UpdatedAt: mr.UpdatedAt, WebURL: mr.WebURL, Draft: mr.Draft,
	}
}
