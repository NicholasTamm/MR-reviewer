package provider

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

var (
	plusLine = regexp.MustCompile(`\+\[L(\d+)\]`)
	plusFile = regexp.MustCompile(`(?m)^\+\+\+ b/(.+)$`)
)

// Echo is an offline provider used by tests and agent dry-runs.
type Echo struct{}

func (Echo) Name() string { return "echo" }

func (Echo) Review(_ context.Context, _, user string) (review.Result, error) {
	file := "unknown"
	if m := plusFile.FindStringSubmatch(user); len(m) == 2 {
		file = strings.TrimSpace(m[1])
	}
	line := 1
	if m := plusLine.FindStringSubmatch(user); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			line = n
		}
	}
	return review.Result{
		Summary: "**APPROVED**\nEcho review: the annotated diff was received and parsed.",
		Comments: []review.Comment{{
			File:     file,
			Line:     line,
			Body:     "*info* **SUGGESTION:** Echo provider comment on the first added line.",
			Severity: "info",
		}},
	}, nil
}
