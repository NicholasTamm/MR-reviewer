REVIEW_SYSTEM_PROMPT = """\
You are an expert code reviewer. Your task is to review merge request changes \
and provide actionable, specific feedback.

Review the code changes below. Focus on: {focus_areas}.

Provide your review using the submit_review tool with:
- A summary of your overall assessment (2-4 sentences)
- Specific inline comments on individual lines that need attention

Guidelines:
- Only comment on lines that are actually changed (additions shown with '+' in the diff)
- Be specific and actionable — explain what's wrong and suggest how to fix it
- Use the correct file path and line number from the diff
- severity "error" = must fix before merging
- severity "warning" = should fix, potential issue
- severity "info" = suggestion for improvement
- Don't comment on every line — focus on meaningful issues
- If the code looks good, say so in the summary and provide few or no inline comments
"""

REVIEW_USER_TEMPLATE = """\
## Merge Request: {title}

{description}

## Diff
```diff
{diff}
```

## Changed File Contents
{file_contents}
"""


def build_system_prompt(focus: list[str]) -> str:
    focus_str = ", ".join(focus)
    return REVIEW_SYSTEM_PROMPT.format(focus_areas=focus_str)


def build_user_message(
    title: str,
    description: str,
    diff: str,
    file_contents: dict[str, str],
) -> str:
    contents_parts = []
    for path, content in file_contents.items():
        contents_parts.append(f"### {path}\n```\n{content}\n```")

    return REVIEW_USER_TEMPLATE.format(
        title=title,
        description=description or "(no description)",
        diff=diff,
        file_contents="\n\n".join(contents_parts),
    )
