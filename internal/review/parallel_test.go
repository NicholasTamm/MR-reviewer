package review

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMergeResultsConcatenatesAndDedups(t *testing.T) {
	merged := MergeResults([]Result{
		{Summary: "First", Comments: []Comment{{File: "foo.py", Line: 10, Body: "first"}}},
		{Summary: "Second", Comments: []Comment{{File: "foo.py", Line: 10, Body: "second"}, {File: "a.py", Line: 3}}},
	})
	if !strings.Contains(merged.Summary, "First") || !strings.Contains(merged.Summary, "Second") || !strings.Contains(merged.Summary, "---") {
		t.Fatalf("summary = %q", merged.Summary)
	}
	if len(merged.Comments) != 2 || merged.Comments[0].File != "a.py" || merged.Comments[1].Body != "first" {
		t.Fatalf("comments = %+v", merged.Comments)
	}
}

func TestMergeResultsEmpty(t *testing.T) {
	got := MergeResults(nil)
	if got.Summary != "" || got.Comments == nil {
		t.Fatalf("%+v", got)
	}
}

func TestParallelReviewSplitsAndMerges(t *testing.T) {
	var calls atomic.Int32
	prov := &countingProvider{fn: func(system, user string) (Result, error) {
		calls.Add(1)
		file := "file0.py"
		if strings.Contains(user, "+++ b/file1.py") {
			file = "file1.py"
		}
		return Result{Summary: "agent", Comments: []Comment{{File: file, Line: 1, Body: "ok"}}}, nil
	}}
	files := []DiffFile{
		{OldPath: "file0.py", NewPath: "file0.py", Diff: "@@ -1 +1 @@\n-old\n+new\n"},
		{OldPath: "file1.py", NewPath: "file1.py", Diff: "@@ -1 +1 @@\n-old\n+new\n"},
	}
	got, err := ParallelReview(context.Background(), prov, files, map[string]string{"file0.py": "new", "file1.py": "new"}, []string{"bugs"}, Metadata{Title: "PR"}, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d", calls.Load())
	}
	if len(got.Comments) != 2 {
		t.Fatalf("comments = %+v", got.Comments)
	}
}

func TestParallelReviewPartialFailure(t *testing.T) {
	var calls atomic.Int32
	prov := &countingProvider{fn: func(system, user string) (Result, error) {
		if calls.Add(1) == 1 {
			return Result{}, errors.New("boom")
		}
		return Result{Summary: "ok", Comments: []Comment{{File: "file0.py", Line: 1}}}, nil
	}}
	files := []DiffFile{
		{OldPath: "a.py", NewPath: "a.py", Diff: "@@ -1 +1 @@\n-x\n+y\n"},
		{OldPath: "b.py", NewPath: "b.py", Diff: "@@ -1 +1 @@\n-x\n+y\n"},
	}
	got, err := ParallelReview(context.Background(), prov, files, nil, nil, Metadata{}, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "ok" {
		t.Fatalf("summary = %q", got.Summary)
	}
}

func TestParallelReviewAllFail(t *testing.T) {
	prov := &countingProvider{fn: func(string, string) (Result, error) {
		return Result{}, errors.New("nope")
	}}
	files := []DiffFile{{OldPath: "a.py", NewPath: "a.py", Diff: "@@ -1 +1 @@\n-x\n+y\n"}}
	_, err := ParallelReview(context.Background(), prov, files, nil, nil, Metadata{}, 2, 10)
	if err == nil || !strings.Contains(err.Error(), "all parallel review agents failed") {
		t.Fatalf("err = %v", err)
	}
}

type countingProvider struct {
	fn func(system, user string) (Result, error)
}

func (c *countingProvider) Name() string { return "count" }

func (c *countingProvider) Review(_ context.Context, system, user string) (Result, error) {
	return c.fn(system, user)
}
