package review

import (
	"context"
	"fmt"
)

// Provider runs one structured review.
type Provider interface {
	Name() string
	Review(ctx context.Context, system, user string) (Result, error)
}

// Platform fetches MR/PR data and posts reviews.
type Platform interface {
	FetchChanges(ctx context.Context, info Info) (FetchResult, error)
	FetchFile(ctx context.Context, info Info, path, ref string) (string, bool, error)
	PostReview(ctx context.Context, info Info, result Result, lines []DiffLine) error
}

// Options configure a single review run.
type Options struct {
	URL               string
	Provider          Provider
	Platform          Platform
	Focus             []string
	MaxComments       int
	DryRun            bool
	Parallel          bool
	ParallelThreshold int
	Progress          func(status, message string)
}

// Outcome is a review plus the data needed to post later.
type Outcome struct {
	Info      Info
	Result    Result
	DiffLines []DiffLine
}

// Run parses the URL, fetches the diff and file contents, asks the provider
// for a review, drops off-diff comments, enforces the comment budget, and
// either dry-runs or posts.
func Run(ctx context.Context, opts Options) (Result, error) {
	out, err := Execute(ctx, opts)
	if err != nil {
		return Result{}, err
	}
	return out.Result, nil
}

// Execute is Run without discarding Info and DiffLines.
func Execute(ctx context.Context, opts Options) (Outcome, error) {
	if opts.Provider == nil {
		return Outcome{}, fmt.Errorf("review provider is required")
	}
	if opts.Platform == nil {
		return Outcome{}, fmt.Errorf("platform client is required")
	}
	maxComments := opts.MaxComments
	if maxComments <= 0 {
		maxComments = 10
	}
	focus := opts.Focus
	if len(focus) == 0 {
		focus = DefaultFocus
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(string, string) {}
	}

	progress("fetching", "Parsing URL...")
	info, err := Parse(opts.URL)
	if err != nil {
		return Outcome{}, err
	}

	progress("fetching", "Fetching MR changes...")
	fetched, err := opts.Platform.FetchChanges(ctx, info)
	if err != nil {
		return Outcome{}, err
	}
	if len(fetched.DiffFiles) == 0 {
		return Outcome{
			Info:   info,
			Result: Result{Summary: "No changes found in this MR.", Comments: []Comment{}, Meta: fetched.Metadata},
		}, nil
	}

	unified := BuildUnifiedDiff(fetched.DiffFiles)
	diffLines := ParseDiff(unified)

	progress("fetching", fmt.Sprintf("Fetching contents for %d files...", len(fetched.DiffFiles)))
	fileContents := map[string]string{}
	ref := fetched.Metadata.SourceBranch
	if ref == "" {
		ref = "HEAD"
	}
	for _, path := range ChangedPaths(unified) {
		content, ok, err := opts.Platform.FetchFile(ctx, info, path, ref)
		if err != nil {
			return Outcome{}, err
		}
		if ok {
			fileContents[path] = content
		}
	}

	progress("reviewing", "Running AI review...")
	threshold := opts.ParallelThreshold
	if threshold <= 0 {
		threshold = 10
	}
	var raw Result
	if opts.Parallel && len(fetched.DiffFiles) >= threshold {
		raw, err = ParallelReview(ctx, opts.Provider, fetched.DiffFiles, fileContents, focus, fetched.Metadata, defaultParallelAgents, maxComments)
	} else {
		system := SystemPrompt(focus, maxComments)
		user := UserMessage(fetched.Metadata.Title, fetched.Metadata.Description, AnnotateDiff(unified), fileContents)
		raw, err = opts.Provider.Review(ctx, system, user)
	}
	if err != nil {
		return Outcome{}, err
	}

	var valid []Comment
	for _, c := range raw.Comments {
		if !ValidateCommentLine(c.File, c.Line, diffLines) {
			continue
		}
		c.IsNewLine = DetermineNewLine(c.File, c.Line, diffLines)
		if c.Severity == "" {
			c.Severity = "info"
		}
		valid = append(valid, c)
	}
	valid = EnforceBudget(valid, maxComments)
	result := Result{Summary: raw.Summary, Comments: valid, Meta: fetched.Metadata}
	if result.Comments == nil {
		result.Comments = []Comment{}
	}

	if !opts.DryRun {
		progress("reviewing", "Posting review...")
		if err := opts.Platform.PostReview(ctx, info, result, diffLines); err != nil {
			return Outcome{}, err
		}
	}
	return Outcome{Info: info, Result: result, DiffLines: diffLines}, nil
}
