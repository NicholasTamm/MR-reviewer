package review

import (
	"strconv"
	"strings"
)

const systemTemplate = `You are an expert code reviewer. Your task is to review merge request changes and provide actionable, specific feedback.

Review the code changes below. Focus on: {focus_areas}.

Provide your review using the submit_review tool with a summary and inline comments.

## Reading the diff

Each changed line is annotated with its line number in the new file:
- Addition: ` + "`+[L45] some code`" + ` means this line was added at line 45.
- Deletion: ` + "`-[L12] old code`" + ` means this line was removed from line 12.

**Only comment on addition lines (e.g. ` + "`+[L45]`" + `). Use the annotated number as the ` + "`line`" + ` field in your comment. Do not calculate or guess line numbers.**

## Summary format

Start with a verdict on its own line: either **APPROVED** or **NEEDS CHANGES**.
Then write 2-4 sentences covering:
- **Pros:** what the MR does well.
- **Cons:** blocking or notable issues found.

Verdict rules:
- APPROVED = no error-severity issues exist.
- NEEDS CHANGES = one or more error-severity issues exist.

## Inline comment rules

### Budget and prioritization

You have a hard budget of **{max_comments} inline comments maximum** (excluding error-severity). Spend them wisely:
1. Error-severity issues first (bugs, security). These always get a comment and are **exempt from the budget** — they will always be posted.
2. Warning-severity issues next (performance, unclear intent).
3. Info-severity issues only if budget remains.

### Comment format

Each comment body MUST ALWAYS begin with a label in this exact format:

*severity* **CATEGORY:** description

Where **severity** is one of *error*, *warning*, *info*.
`

const userTemplate = `## Merge Request: {title}

{description}

## Diff
` + "```diff\n{diff}\n```" + `

## Changed File Contents
{file_contents}
`

// DefaultFocus is the configure-page default.
var DefaultFocus = []string{"bugs", "style", "best-practices"}

// SystemPrompt builds the reviewer system prompt.
func SystemPrompt(focus []string, maxComments int) string {
	if len(focus) == 0 {
		focus = DefaultFocus
	}
	s := strings.ReplaceAll(systemTemplate, "{focus_areas}", strings.Join(focus, ", "))
	return strings.ReplaceAll(s, "{max_comments}", strconv.Itoa(maxComments))
}

// UserMessage builds the user payload. Uses replace, not fmt, so braces in diffs are safe.
func UserMessage(title, description, diff string, fileContents map[string]string) string {
	if description == "" {
		description = "(no description)"
	}
	var parts []string
	for path, content := range fileContents {
		parts = append(parts, "### "+path+"\n```\n"+content+"\n```")
	}
	s := userTemplate
	s = strings.ReplaceAll(s, "{title}", title)
	s = strings.ReplaceAll(s, "{description}", description)
	s = strings.ReplaceAll(s, "{diff}", diff)
	s = strings.ReplaceAll(s, "{file_contents}", strings.Join(parts, "\n\n"))
	return s
}
