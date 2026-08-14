package review

import (
	"strings"
	"testing"
)

func TestSystemPromptInsertsFocusAndBudget(t *testing.T) {
	got := SystemPrompt([]string{"security", "bugs"}, 4)
	if !strings.Contains(got, "security, bugs") {
		t.Fatalf("missing focus: %s", got)
	}
	if !strings.Contains(got, "4 inline comments") {
		t.Fatalf("missing budget: %s", got)
	}
	if !strings.Contains(got, "submit_review") {
		t.Fatalf("missing tool: %s", got)
	}
}

func TestSystemPromptEmptyFocusUsesDefault(t *testing.T) {
	got := SystemPrompt(nil, 10)
	for _, f := range DefaultFocus {
		if !strings.Contains(got, f) {
			t.Fatalf("missing default focus %q in %s", f, got)
		}
	}
}

func TestUserMessageReplacesFieldsAndKeepsBraces(t *testing.T) {
	got := UserMessage("Title {x}", "", "@@\n+func {return}()", map[string]string{
		"a.go": "package p\nfunc f() { return }\n",
	})
	if !strings.Contains(got, "Title {x}") {
		t.Fatalf("title lost: %s", got)
	}
	if !strings.Contains(got, "(no description)") {
		t.Fatalf("empty desc: %s", got)
	}
	if !strings.Contains(got, "+func {return}()") {
		t.Fatalf("diff braces broken: %s", got)
	}
	if !strings.Contains(got, "### a.go") || !strings.Contains(got, "func f() { return }") {
		t.Fatalf("file contents: %s", got)
	}
}

func TestUserMessageUsesDescription(t *testing.T) {
	got := UserMessage("T", "does the thing", "diff-body", nil)
	if !strings.Contains(got, "does the thing") || strings.Contains(got, "(no description)") {
		t.Fatalf("%s", got)
	}
	if !strings.Contains(got, "```diff\ndiff-body\n```") {
		t.Fatalf("diff fence: %s", got)
	}
}
