import os
import sys


class Config:
    """Configuration loaded from environment variables."""

    def __init__(self) -> None:
        self.gitlab_token: str = os.environ.get("GITLAB_TOKEN", "")
        self.anthropic_api_key: str = os.environ.get("ANTHROPIC_API_KEY", "")
        self.default_model: str = os.environ.get(
            "MR_REVIEWER_MODEL", "claude-sonnet-4-20250514"
        )
        self.default_focus: list[str] = os.environ.get(
            "MR_REVIEWER_FOCUS", "bugs,style,best-practices"
        ).split(",")

    def require_gitlab_token(self) -> str:
        if not self.gitlab_token:
            print(
                "Error: GITLAB_TOKEN environment variable is not set.\n"
                "Create a GitLab Personal Access Token with 'api' scope and export it:\n"
                "  export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx",
                file=sys.stderr,
            )
            sys.exit(1)
        return self.gitlab_token

    def require_anthropic_key(self) -> str:
        if not self.anthropic_api_key:
            print(
                "Error: ANTHROPIC_API_KEY environment variable is not set.\n"
                "Get your API key from https://console.anthropic.com/ and export it:\n"
                "  export ANTHROPIC_API_KEY=sk-ant-xxxxxxxxxxxxxxxxxxxx",
                file=sys.stderr,
            )
            sys.exit(1)
        return self.anthropic_api_key
