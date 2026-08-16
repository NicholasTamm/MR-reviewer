package review

import "testing"

func cmt(file string, line int, sev string) Comment {
	return Comment{File: file, Line: line, Body: sev + " issue", Severity: sev}
}

func TestBudgetUnderLimitUnchanged(t *testing.T) {
	in := []Comment{cmt("a.py", 1, "warning"), cmt("a.py", 2, "info")}
	got := EnforceBudget(in, 5)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestBudgetTruncatesNonCritical(t *testing.T) {
	in := []Comment{
		cmt("a.py", 1, "error"),
		cmt("a.py", 2, "error"),
		cmt("a.py", 3, "warning"),
		cmt("a.py", 4, "warning"),
		cmt("a.py", 5, "info"),
		cmt("a.py", 6, "info"),
		cmt("a.py", 7, "info"),
	}
	got := EnforceBudget(in, 5)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	errors := 0
	for _, c := range got {
		if c.Severity == "error" {
			errors++
		}
	}
	if errors != 2 {
		t.Fatalf("errors = %d", errors)
	}
}

func TestBudgetErrorsAlwaysKept(t *testing.T) {
	in := make([]Comment, 8)
	for i := range in {
		in[i] = cmt("a.py", i, "error")
	}
	got := EnforceBudget(in, 3)
	if len(got) != 8 {
		t.Fatalf("len = %d, want all 8 errors", len(got))
	}
}

func TestBudgetWarningsBeforeInfo(t *testing.T) {
	in := []Comment{
		cmt("a.py", 1, "info"),
		cmt("a.py", 2, "warning"),
		cmt("a.py", 3, "info"),
		cmt("a.py", 4, "warning"),
	}
	got := EnforceBudget(in, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	for _, c := range got {
		if c.Severity != "warning" {
			t.Fatalf("kept %q, want warning", c.Severity)
		}
	}
}

func TestBudgetSortedByFileAndLine(t *testing.T) {
	in := []Comment{
		cmt("b.py", 10, "error"),
		cmt("a.py", 5, "warning"),
		cmt("a.py", 1, "error"),
	}
	got := EnforceBudget(in, 10)
	want := [][2]any{{"a.py", 1}, {"a.py", 5}, {"b.py", 10}}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	for i, w := range want {
		if got[i].File != w[0] || got[i].Line != w[1] {
			t.Fatalf("got[%d] = %s:%d, want %v", i, got[i].File, got[i].Line, w)
		}
	}
}
