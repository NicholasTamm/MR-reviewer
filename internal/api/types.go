package api

import "time"

type reviewRequest struct {
	URL         string   `json:"url"`
	Provider    string   `json:"provider"`
	Model       string   `json:"model"`
	Focus       []string `json:"focus"`
	MaxComments int      `json:"max_comments"`
	Parallel    bool     `json:"parallel"`
	AutoPost    bool     `json:"auto_post"`
}

type jobStatus struct {
	JobID     string  `json:"job_id"`
	Status    string  `json:"status"`
	Progress  *string `json:"progress"`
	Error     *string `json:"error"`
	ErrorType *string `json:"error_type"`
	CreatedAt string  `json:"created_at"`
	URL       string  `json:"url"`
}

type metadataResponse struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`
}

type commentDetail struct {
	ID          string   `json:"id"`
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Body        string   `json:"body"`
	Severity    string   `json:"severity"`
	IsNewLine   bool     `json:"is_new_line"`
	DiffContext []string `json:"diff_context"`
	Approved    bool     `json:"approved"`
}

type reviewResponse struct {
	JobID    string           `json:"job_id"`
	Summary  string           `json:"summary"`
	Comments []commentDetail  `json:"comments"`
	Metadata metadataResponse `json:"metadata"`
}

type postRequest struct {
	CommentIDs []string `json:"comment_ids"`
	Summary    string   `json:"summary"`
}

type commentEditRequest struct {
	Body string `json:"body"`
}

type configDefaults struct {
	Provider          string   `json:"provider"`
	Model             *string  `json:"model"`
	Focus             []string `json:"focus"`
	MaxComments       int      `json:"max_comments"`
	Parallel          bool     `json:"parallel"`
	ParallelThreshold int      `json:"parallel_threshold"`
}

type providerModelsResponse struct {
	Provider  string   `json:"provider"`
	Models    []string `json:"models"`
	Available bool     `json:"available"`
	Error     *string  `json:"error"`
}

type providerCatalogResponse struct {
	Providers []providerModelsResponse `json:"providers"`
}

type gitlabMRJSON struct {
	ProjectID    int    `json:"project_id"`
	ProjectPath  string `json:"project_path"`
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	UpdatedAt    string `json:"updated_at"`
	WebURL       string `json:"web_url"`
	Draft        bool   `json:"draft"`
}

type gitlabProjectMRsJSON struct {
	ProjectID     int            `json:"project_id"`
	ProjectPath   string         `json:"project_path"`
	MergeRequests []gitlabMRJSON `json:"merge_requests"`
}

type gitlabMRCatalogJSON struct {
	Projects []gitlabProjectMRsJSON `json:"projects"`
}

type gitlabProjectJSON struct {
	ProjectID   int    `json:"project_id"`
	ProjectPath string `json:"project_path"`
	WebURL      string `json:"web_url"`
}

type gitlabProjectsJSON struct {
	Projects []gitlabProjectJSON `json:"projects"`
}

type githubProjectJSON struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	WebURL   string `json:"web_url"`
	Platform string `json:"platform"`
}

type githubProjectsJSON struct {
	Projects []githubProjectJSON `json:"projects"`
}

type githubPRJSON struct {
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	UpdatedAt    string `json:"updated_at"`
	WebURL       string `json:"web_url"`
	Draft        bool   `json:"draft"`
}

type githubProjectPRsJSON struct {
	ID           string         `json:"id"`
	Path         string         `json:"path"`
	WebURL       string         `json:"web_url"`
	PullRequests []githubPRJSON `json:"pull_requests"`
}

type onboardingOptionJSON struct {
	ID            string   `json:"id"`
	HasCredential bool     `json:"has_credential"`
	Methods       []string `json:"methods"`
}

type onboardingStatusJSON struct {
	Complete         bool                   `json:"complete"`
	Reason           string                 `json:"reason"`
	Repair           bool                   `json:"repair"`
	SelectedProvider string                 `json:"selected_provider"`
	SelectedPlatform string                 `json:"selected_platform"`
	Providers        []onboardingOptionJSON `json:"providers"`
	Platforms        []onboardingOptionJSON `json:"platforms"`
}

type onboardingCompleteRequest struct {
	Provider string `json:"provider"`
	Platform string `json:"platform"`
}

type onboardingSecretRequest struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

type authStartRequest struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Method string `json:"method"`
}

type authSessionJSON struct {
	SessionID               string `json:"session_id"`
	Kind                    string `json:"kind"`
	Name                    string `json:"name"`
	Method                  string `json:"method"`
	Status                  string `json:"status"`
	UserCode                string `json:"user_code,omitempty"`
	VerificationURI         string `json:"verification_uri,omitempty"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	AuthorizationURL        string `json:"authorization_url,omitempty"`
	Error                   string `json:"error,omitempty"`
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
