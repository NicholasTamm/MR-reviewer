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
	URL         string
	Provider    Provider
	Platform    Platform
	Focus       []string
	MaxComments int
	DryRun      bool
}

// Run parses the URL, fetches the diff and file contents, asks the provider
// for a review, drops off-diff comments, enforces the comment budget, and
// either dry-runs or posts.
func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.Provider == nil {
		return Result{}, fmt.Errorf("review provider is required")
	}
	if opts.Platform == nil {
		return Result{}, fmt.Errorf("platform client is required")
	}
	maxComments := opts.MaxComments
	if maxComments <= 0 {
		maxComments = 10
	}
	focus := opts.Focus
	if len(focus) == 0 {
		focus = DefaultFocus
	}

	info, err := Parse(opts.URL)
	if err != nil {
		return Result{}, err
	}

	fetched, err := opts.Platform.FetchChanges(ctx, info)
	if err != nil {
		return Result{}, err
	}
	if len(fetched.DiffFiles) == 0 {
		return Result{Summary: "No changes found in this MR.", Comments: []Comment{}, Meta: fetched.Metadata}, nil
	}

	unified := BuildUnifiedDiff(fetched.DiffFiles)
	diffLines := ParseDiff(unified)

	fileContents := map[string]string{}
	ref := fetched.Metadata.SourceBranch
	if ref == "" {
		ref = "HEAD"
	}
	for _, path := range ChangedPaths(unified) {
		content, ok, err := opts.Platform.FetchFile(ctx, info, path, ref)
		if err != nil {
			return Result{}, err
		}
		if ok {
			fileContents[path] = content
		}
	}

	system := SystemPrompt(focus, maxComments)
	user := UserMessage(fetched.Metadata.Title, fetched.Metadata.Description, AnnotateDiff(unified), fileContents)

	raw, err := opts.Provider.Review(ctx, system, user)
	if err != nil {
		return Result{}, err
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
	out := Result{Summary: raw.Summary, Comments: valid, Meta: fetched.Metadata}
	if out.Comments == nil {
		out.Comments = []Comment{}
	}

	if !opts.DryRun {
		if err := opts.Platform.PostReview(ctx, info, out, diffLines); err != nil {
			return Result{}, err
		}
	}
	return out, nil
}
