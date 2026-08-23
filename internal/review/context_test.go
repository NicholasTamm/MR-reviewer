package review

import "testing"

func TestExtractDiffContextWindow(t *testing.T) {
	n2, n3 := 2, 3
	lines := []DiffLine{
		{FilePath: "hello.py", NewLine: &n2, LineType: "+", Content: "import sys"},
		{FilePath: "hello.py", NewLine: &n3, LineType: "+", Content: "import json"},
		{FilePath: "other.py", NewLine: &n2, LineType: "+", Content: "skip"},
	}
	got := ExtractDiffContext("hello.py", 2, lines, 3)
	if len(got) != 2 || got[0] != "+import sys" || got[1] != "+import json" {
		t.Fatalf("%q", got)
	}
}

func TestExtractDiffContextMissing(t *testing.T) {
	if got := ExtractDiffContext("missing.py", 1, ParseDiff(diffAdditionsOnly), 3); len(got) != 0 {
		t.Fatalf("%q", got)
	}
	if got := ExtractDiffContext("hello.py", 999, ParseDiff(diffAdditionsOnly), 3); len(got) != 0 {
		t.Fatalf("%q", got)
	}
}
