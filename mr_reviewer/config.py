import os

from mr_reviewer.exceptions import ConfigurationError

DEFAULT_MODEL = "claude-sonnet-4-20250514"
DEFAULT_FOCUS = ["bugs", "style", "best-practices"]


class Config:
    """Configuration loaded from environment variables."""

    def __init__(self) -> None:
        self.gitlab_token: str = os.environ.get("GITLAB_TOKEN", "")
        self.anthropic_api_key: str = os.environ.get("ANTHROPIC_API_KEY", "")
        self.model: str = os.environ.get(
            "MR_REVIEWER_MODEL", DEFAULT_MODEL
        )
        self.default_focus: list[str] = os.environ.get(
            "MR_REVIEWER_FOCUS", ",".join(DEFAULT_FOCUS)
        ).split(",")
        self.provider: str = os.environ.get("MR_REVIEWER_PROVIDER", "anthropic")
        self.gemini_api_key: str = os.environ.get("GEMINI_API_KEY", "")
        self.ollama_host: str = os.environ.get(
            "OLLAMA_HOST", "http://localhost:11434"
        )
        self.github_token: str = os.environ.get("GITHUB_TOKEN", "")
        self.parallel_review: bool = os.environ.get(
            "MR_REVIEWER_PARALLEL", ""
        ).lower() in ("1", "true", "yes")
        self.parallel_threshold: int = int(
            os.environ.get("MR_REVIEWER_PARALLEL_THRESHOLD", "10")
        )
        self.max_comments: int = int(
            os.environ.get("MR_REVIEWER_MAX_COMMENTS", "10")
        )

        self._load_from_keyring()

    def _load_from_keyring(self) -> None:
        """Overlay credentials from OS keychain (keyring). Env vars take precedence."""
        try:
            import keyring  # optional dependency  # noqa: PLC0415
            service = 'mr-reviewer'
            keys_to_attrs = {
                'GITLAB_TOKEN': 'gitlab_token',
                'GITHUB_TOKEN': 'github_token',
                'ANTHROPIC_API_KEY': 'anthropic_api_key',
                'GEMINI_API_KEY': 'gemini_api_key',
                'OLLAMA_HOST': 'ollama_host',
            }
            for env_key, attr in keys_to_attrs.items():
                if not getattr(self, attr, None):  # only if not set by env var
                    value = keyring.get_password(service, env_key)
                    if value:
                        setattr(self, attr, value)
        except Exception:
            pass  # keyring not available or keychain inaccessible — silent fallback

    def require_gitlab_token(self) -> str:
        if not self.gitlab_token:
            raise ConfigurationError(
                "Error: GITLAB_TOKEN environment variable is not set.\n"
                "Create a GitLab Personal Access Token with 'api' scope and export it:\n"
                "  export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx"
            )
        return self.gitlab_token

    def require_anthropic_key(self) -> str:
        if not self.anthropic_api_key:
            raise ConfigurationError(
                "Error: ANTHROPIC_API_KEY environment variable is not set.\n"
                "Get your API key from https://console.anthropic.com/ and export it:\n"
                "  export ANTHROPIC_API_KEY=sk-ant-xxxxxxxxxxxxxxxxxxxx"
            )
        return self.anthropic_api_key

    def require_github_token(self) -> str:
        if not self.github_token:
            raise ConfigurationError(
                "Error: GITHUB_TOKEN environment variable is not set.\n"
                "Create a GitHub Personal Access Token and export it:\n"
                "  export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx"
            )
        return self.github_token

    def require_gemini_key(self) -> str:
        if not self.gemini_api_key:
            raise ConfigurationError(
                "Error: GEMINI_API_KEY environment variable is not set.\n"
                "Get your API key from https://aistudio.google.com/ and export it:\n"
                "  export GEMINI_API_KEY=your-gemini-api-key"
            )
        return self.gemini_api_key
