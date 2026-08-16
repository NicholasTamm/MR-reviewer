package review

import (
	"strings"
	"testing"
)

const diffAdditionsOnly = `--- a/hello.py
+++ b/hello.py
@@ -1,3 +1,5 @@
 import os
+import sys
+import json

 def main():
`

const diffDeletionsOnly = `--- a/hello.py
+++ b/hello.py
@@ -1,5 +1,3 @@
 import os
-import sys
-import json

 def main():
`

const diffMixed = `--- a/hello.py
+++ b/hello.py
@@ -1,4 +1,4 @@
 import os
-import sys
+import json

 def main():
`

const diffMultiFile = `--- a/foo.py
+++ b/foo.py
@@ -1,3 +1,4 @@
 import os
+import sys

 def foo():
--- a/bar.py
+++ b/bar.py
@@ -1,3 +1,4 @@
 import json
+import csv

 def bar():
`

func TestParseDiffAdditionsOnly(t *testing.T) {
	lines := ParseDiff(diffAdditionsOnly)
	if len(lines) != 2 {
		t.Fatalf("len = %d, want 2: %+v", len(lines), lines)
	}
	for _, dl := range lines {
		if dl.LineType != "+" {
			t.Fatalf("line type %q", dl.LineType)
		}
	}
	if lines[0].NewLine == nil || *lines[0].NewLine != 2 {
		t.Fatalf("first new_line = %v", lines[0].NewLine)
	}
	if lines[1].NewLine == nil || *lines[1].NewLine != 3 {
		t.Fatalf("second new_line = %v", lines[1].NewLine)
	}
}

func TestParseDiffDeletionsOnly(t *testing.T) {
	lines := ParseDiff(diffDeletionsOnly)
	if len(lines) != 2 {
		t.Fatalf("len = %d", len(lines))
	}
	for _, dl := range lines {
		if dl.LineType != "-" {
			t.Fatalf("line type %q", dl.LineType)
		}
	}
	if lines[0].OldLine == nil || *lines[0].OldLine != 2 {
		t.Fatalf("first old_line = %v", lines[0].OldLine)
	}
	if lines[1].OldLine == nil || *lines[1].OldLine != 3 {
		t.Fatalf("second old_line = %v", lines[1].OldLine)
	}
}

func TestParseDiffMixed(t *testing.T) {
	lines := ParseDiff(diffMixed)
	if len(lines) != 2 {
		t.Fatalf("len = %d", len(lines))
	}
	types := map[string]bool{}
	for _, dl := range lines {
		types[dl.LineType] = true
	}
	if !types["+"] || !types["-"] {
		t.Fatalf("types = %v", types)
	}
}

func TestParseDiffEmpty(t *testing.T) {
	if lines := ParseDiff(""); len(lines) != 0 {
		t.Fatalf("%+v", lines)
	}
}

func TestParseDiffInvalid(t *testing.T) {
	if lines := ParseDiff("this is not a diff"); len(lines) != 0 {
		t.Fatalf("%+v", lines)
	}
}

func TestValidateCommentLinePositive(t *testing.T) {
	lines := ParseDiff(diffAdditionsOnly)
	if !ValidateCommentLine("hello.py", 2, lines) {
		t.Fatal("expected valid addition line")
	}
}

func TestValidateCommentLineRejectsOldLine(t *testing.T) {
	lines := ParseDiff(diffDeletionsOnly)
	if ValidateCommentLine("hello.py", 2, lines) {
		t.Fatal("deletion-only line should be rejected")
	}
}

func TestValidateCommentLineNegative(t *testing.T) {
	lines := ParseDiff(diffAdditionsOnly)
	if ValidateCommentLine("hello.py", 999, lines) {
		t.Fatal("hallucinated line should be rejected")
	}
}

func TestChangedPathsMultipleFiles(t *testing.T) {
	paths := ChangedPaths(diffMultiFile)
	if len(paths) != 2 {
		t.Fatalf("paths = %v", paths)
	}
	joined := strings.Join(paths, ",")
	if !strings.Contains(joined, "foo.py") || !strings.Contains(joined, "bar.py") {
		t.Fatalf("paths = %v", paths)
	}
}

func TestBuildUnifiedDiff(t *testing.T) {
	got := BuildUnifiedDiff([]DiffFile{{
		OldPath: "src/main.py",
		NewPath: "src/main.py",
		Diff:    "@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
	}})
	if !strings.Contains(got, "--- a/src/main.py") || !strings.Contains(got, "+++ b/src/main.py") || !strings.Contains(got, "+import sys") {
		t.Fatalf("got %q", got)
	}
}

func TestBuildUnifiedDiffSkipsBinaryAndEmpty(t *testing.T) {
	got := BuildUnifiedDiff([]DiffFile{
		{OldPath: "image.png", NewPath: "image.png", IsBinary: true},
		{OldPath: "empty.py", NewPath: "empty.py", Diff: ""},
	})
	if got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestAnnotateDiffAddsLineNumbers(t *testing.T) {
	got := AnnotateDiff(diffAdditionsOnly)
	if !strings.Contains(got, "+[L2]import sys") || !strings.Contains(got, "+[L3]import json") {
		t.Fatalf("annotated = %q", got)
	}
}
