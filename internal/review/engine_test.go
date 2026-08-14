package review

import (
	"context"
	"errors"
	"testing"
)

type fakePlatform struct {
	fetch   FetchResult
	files   map[string]string
	posted  *Result
	postErr error
}

func (f *fakePlatform) FetchChanges(context.Context, Info) (FetchResult, error) {
	return f.fetch, nil
}

func (f *fakePlatform) FetchFile(_ context.Context, _ Info, path, _ string) (string, bool, error) {
	c, ok := f.files[path]
	return c, ok, nil
}

func (f *fakePlatform) PostReview(_ context.Context, _ Info, result Result, _ []DiffLine) error {
	cp := result
	f.posted = &cp
	return f.postErr
}

type fakeProvider struct {
	result Result
	err    error
	calls  int
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Review(context.Context, string, string) (Result, error) {
	f.calls++
	return f.result, f.err
}

func sampleFetch() FetchResult {
	return FetchResult{
		DiffFiles: []DiffFile{{
			OldPath: "main.py",
			NewPath: "main.py",
			Diff:    "@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
		}},
		Metadata: Metadata{
			Title:        "Test MR",
			Description:  "A test",
			SourceBranch: "feature",
			TargetBranch: "main",
			WebURL:       "https://github.com/owner/repo/pull/1",
		},
	}
}

func TestRunDryRunDoesNotPost(t *testing.T) {
	plat := &fakePlatform{
		fetch: sampleFetch(),
		files: map[string]string{"main.py": "import os\nimport sys\n"},
	}
	prov := &fakeProvider{result: Result{
		Summary: "Looks good.",
		Comments: []Comment{{
			File: "main.py", Line: 2, Body: "Consider a constant.", Severity: "info",
		}},
	}}
	got, err := Run(context.Background(), Options{
		URL: "https://github.com/owner/repo/pull/1", Provider: prov, Platform: plat, DryRun: true, MaxComments: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "Looks good." || len(got.Comments) != 1 {
		t.Fatalf("%+v", got)
	}
	if !got.Comments[0].IsNewLine {
		t.Fatal("expected addition line")
	}
	if plat.posted != nil {
		t.Fatal("dry-run posted")
	}
	if prov.calls != 1 {
		t.Fatalf("provider calls = %d", prov.calls)
	}
}

func TestRunPostsWhenNotDryRun(t *testing.T) {
	plat := &fakePlatform{
		fetch: sampleFetch(),
		files: map[string]string{"main.py": "import os\nimport sys\n"},
	}
	prov := &fakeProvider{result: Result{
		Summary:  "Looks good.",
		Comments: []Comment{{File: "main.py", Line: 2, Body: "n", Severity: "info"}},
	}}
	got, err := Run(context.Background(), Options{
		URL: "https://github.com/owner/repo/pull/1", Provider: prov, Platform: plat, DryRun: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plat.posted == nil || plat.posted.Summary != got.Summary {
		t.Fatalf("posted = %+v", plat.posted)
	}
}

func TestRunNoChangesSkipsProvider(t *testing.T) {
	plat := &fakePlatform{fetch: FetchResult{Metadata: Metadata{Title: "Empty"}}}
	prov := &fakeProvider{}
	got, err := Run(context.Background(), Options{
		URL: "https://gitlab.com/group/project/-/merge_requests/1", Provider: prov, Platform: plat,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "No changes found in this MR." {
		t.Fatalf("summary = %q", got.Summary)
	}
	if prov.calls != 0 {
		t.Fatal("provider should not run")
	}
}

func TestRunDropsOffDiffCommentsKeepsOnDiff(t *testing.T) {
	plat := &fakePlatform{
		fetch: sampleFetch(),
		files: map[string]string{"main.py": "import os\nimport sys\n"},
	}
	prov := &fakeProvider{result: Result{
		Summary: "Found issues.",
		Comments: []Comment{
			{File: "main.py", Line: 2, Body: "on diff", Severity: "warning"},
			{File: "main.py", Line: 999, Body: "hallucinated", Severity: "error"},
			{File: "other.py", Line: 1, Body: "wrong file", Severity: "info"},
		},
	}}
	got, err := Run(context.Background(), Options{
		URL: "https://github.com/owner/repo/pull/1", Provider: prov, Platform: plat, DryRun: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Comments) != 1 || got.Comments[0].Line != 2 || got.Comments[0].Body != "on diff" {
		t.Fatalf("comments = %+v", got.Comments)
	}
}

func TestRunBudgetKeepsErrors(t *testing.T) {
	// Build a diff with many added lines so error comments stay on-diff.
	var hunk string
	hunk += "@@ -1,1 +1,20 @@\n context\n"
	for i := 2; i <= 20; i++ {
		hunk += "+line\n"
	}
	plat := &fakePlatform{
		fetch: FetchResult{
			DiffFiles: []DiffFile{{OldPath: "a.py", NewPath: "a.py", Diff: hunk}},
			Metadata:  Metadata{Title: "big", SourceBranch: "f"},
		},
		files: map[string]string{"a.py": "x"},
	}
	var comments []Comment
	for i := 2; i <= 10; i++ {
		sev := "info"
		if i <= 4 {
			sev = "error"
		}
		comments = append(comments, Comment{File: "a.py", Line: i, Body: sev, Severity: sev})
	}
	prov := &fakeProvider{result: Result{Summary: "many", Comments: comments}}
	got, err := Run(context.Background(), Options{
		URL: "https://github.com/owner/repo/pull/1", Provider: prov, Platform: plat, DryRun: true, MaxComments: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	errors := 0
	for _, c := range got.Comments {
		if c.Severity == "error" {
			errors++
		}
	}
	if errors != 3 {
		t.Fatalf("errors = %d comments=%+v", errors, got.Comments)
	}
	if len(got.Comments) != 4 {
		t.Fatalf("len = %d, want 4 (3 errors + 1 budget)", len(got.Comments))
	}
}

func TestRunRequiresProviderAndPlatform(t *testing.T) {
	if _, err := Run(context.Background(), Options{URL: "https://github.com/o/r/pull/1", Platform: &fakePlatform{}}); err == nil {
		t.Fatal("expected missing provider")
	}
	if _, err := Run(context.Background(), Options{URL: "https://github.com/o/r/pull/1", Provider: &fakeProvider{}}); err == nil {
		t.Fatal("expected missing platform")
	}
}

func TestRunInvalidURL(t *testing.T) {
	_, err := Run(context.Background(), Options{
		URL: "https://example.com/not-a-pr", Provider: &fakeProvider{}, Platform: &fakePlatform{},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunPostError(t *testing.T) {
	plat := &fakePlatform{
		fetch:   sampleFetch(),
		files:   map[string]string{"main.py": "x"},
		postErr: errors.New("boom"),
	}
	prov := &fakeProvider{result: Result{Summary: "ok"}}
	_, err := Run(context.Background(), Options{
		URL: "https://github.com/owner/repo/pull/1", Provider: prov, Platform: plat,
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("err = %v", err)
	}
}
