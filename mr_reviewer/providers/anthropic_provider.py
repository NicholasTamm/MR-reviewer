import logging

import anthropic

from mr_reviewer.exceptions import ProviderError
from mr_reviewer.models import ReviewComment, ReviewResult

logger = logging.getLogger(__name__)

REVIEW_TOOL = {
    "name": "submit_review",
    "description": "Submit a structured code review with a summary and inline comments",
    "input_schema": {
        "type": "object",
        "properties": {
            "summary": {
                "type": "string",
                "description": "Overall review summary (2-4 sentences)",
            },
            "comments": {
                "type": "array",
                "description": "List of inline comments on specific lines",
                "items": {
                    "type": "object",
                    "properties": {
                        "file": {
                            "type": "string",
                            "description": "File path (e.g., src/main.py)",
                        },
                        "line": {
                            "type": "integer",
                            "description": "Line number in the new version of the file",
                        },
                        "body": {
                            "type": "string",
                            "description": "Comment text (Markdown supported)",
                        },
                        "severity": {
                            "type": "string",
                            "enum": ["info", "warning", "error"],
                            "description": "error=must fix, warning=should fix, info=suggestion",
                        },
                    },
                    "required": ["file", "line", "body", "severity"],
                },
            },
        },
        "required": ["summary", "comments"],
    },
}


class AnthropicProvider:
    """AI review provider using Anthropic's Claude API.

    Thread safety: anthropic.Anthropic uses httpx.Client internally which is
    thread-safe for concurrent messages.create() calls from multiple threads.
    This provider is safe for use in parallel.py (v3).
    """

    def __init__(self, api_key: str, model: str) -> None:
        self._client = anthropic.Anthropic(api_key=api_key)
        self._model = model

    def run_review(self, system_prompt: str, user_message: str) -> ReviewResult:
        """Send the diff and context to Claude and get a structured review back."""
        logger.info("Sending review request to Claude (%s)...", self._model)

        try:
            response = self._client.messages.create(
                model=self._model,
                max_tokens=4096,
                system=system_prompt,
                messages=[{"role": "user", "content": user_message}],
                tools=[REVIEW_TOOL],
                tool_choice={"type": "tool", "name": "submit_review"},
            )
        except anthropic.APIError as e:
            raise ProviderError(f"Anthropic API error: {e}") from e

        # Extract the tool_use block
        tool_block = None
        for block in response.content:
            if block.type == "tool_use":
                tool_block = block
                break

        if tool_block is None:
            logger.error("Claude did not return a tool_use block")
            return ReviewResult(
                summary="Error: Claude did not return a structured review.",
                comments=[],
            )

        data = tool_block.input

        comments = [
            ReviewComment(
                file=c["file"],
                line=c["line"],
                body=c["body"],
                severity=c.get("severity", "info"),
            )
            for c in data.get("comments", [])
        ]

        return ReviewResult(
            summary=data.get("summary", ""),
            comments=comments,
        )
