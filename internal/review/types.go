package review

// Info is a parsed GitHub PR or GitLab MR URL.
type Info struct {
	Platform  string
	Host      string
	BaseURL   string
	Namespace string
	Project   string
	IID       int
}

// DiffFile is one file's change set from the platform.
type DiffFile struct {
	OldPath     string
	NewPath     string
	Diff        string
	NewFile     bool
	RenamedFile bool
	DeletedFile bool
	IsBinary    bool
}

// DiffLine is one added or removed line in a unified diff.
type DiffLine struct {
	FilePath string
	OldLine  *int
	NewLine  *int
	LineType string // "+", "-"
	Content  string
}

// Comment is one inline review comment.
type Comment struct {
	File      string `json:"file"`
	Line      int    `json:"line"`
	Body      string `json:"body"`
	Severity  string `json:"severity"`
	IsNewLine bool   `json:"-"`
}

// Result is the complete review outcome.
type Result struct {
	Summary  string    `json:"summary"`
	Comments []Comment `json:"comments"`
	Meta     Metadata  `json:"-"`
}

// Metadata is MR/PR title and branch info.
type Metadata struct {
	Title        string
	Description  string
	SourceBranch string
	TargetBranch string
	WebURL       string
}

// FetchResult is the platform fetch payload.
type FetchResult struct {
	DiffFiles []DiffFile
	Metadata  Metadata
}

// ProjectSummary is a GitLab project the token can see.
type ProjectSummary struct {
	ProjectID   int
	ProjectPath string
	WebURL      string
}

// MergeRequestSummary is one open GitLab MR.
type MergeRequestSummary struct {
	ProjectID    int
	ProjectPath  string
	IID          int
	Title        string
	Author       string
	SourceBranch string
	TargetBranch string
	UpdatedAt    string
	WebURL       string
	Draft        bool
}

// ProjectMergeRequests groups open MRs under one project.
type ProjectMergeRequests struct {
	ProjectID     int
	ProjectPath   string
	MergeRequests []MergeRequestSummary
}
