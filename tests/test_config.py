import pytest

from mr_reviewer.config import DEFAULT_FOCUS, DEFAULT_MODEL, Config
from mr_reviewer.exceptions import ConfigurationError


def test_config_loads_from_env(monkeypatch):
    monkeypatch.setenv("GITLAB_TOKEN", "test-token")
    monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")
    monkeypatch.setenv("MR_REVIEWER_MODEL", "claude-opus-4-20250514")
    monkeypatch.setenv("MR_REVIEWER_FOCUS", "security,performance")
    monkeypatch.setenv("MR_REVIEWER_PROVIDER", "gemini")
    monkeypatch.setenv("GEMINI_API_KEY", "gemini-key")
    monkeypatch.setenv("OLLAMA_HOST", "http://remote:11434")
    config = Config()
    assert config.gitlab_token == "test-token"
    assert config.anthropic_api_key == "test-key"
    assert config.model == "claude-opus-4-20250514"
    assert config.default_focus == ["security", "performance"]
    assert config.provider == "gemini"
    assert config.gemini_api_key == "gemini-key"
    assert config.ollama_host == "http://remote:11434"


def test_config_uses_defaults(monkeypatch):
    monkeypatch.delenv("GITLAB_TOKEN", raising=False)
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    monkeypatch.delenv("MR_REVIEWER_MODEL", raising=False)
    monkeypatch.delenv("MR_REVIEWER_FOCUS", raising=False)
    monkeypatch.delenv("MR_REVIEWER_PROVIDER", raising=False)
    monkeypatch.delenv("GEMINI_API_KEY", raising=False)
    monkeypatch.delenv("OLLAMA_HOST", raising=False)
    config = Config()
    assert config.gitlab_token == ""
    assert config.anthropic_api_key == ""
    assert config.model == DEFAULT_MODEL
    assert config.default_focus == DEFAULT_FOCUS
    assert config.provider == "anthropic"
    assert config.gemini_api_key == ""
    assert config.ollama_host == "http://localhost:11434"


def test_require_gitlab_token_raises_when_empty(monkeypatch):
    monkeypatch.delenv("GITLAB_TOKEN", raising=False)
    config = Config()
    with pytest.raises(ConfigurationError, match="GITLAB_TOKEN"):
        config.require_gitlab_token()


def test_require_anthropic_key_raises_when_empty(monkeypatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)
    config = Config()
    with pytest.raises(ConfigurationError, match="ANTHROPIC_API_KEY"):
        config.require_anthropic_key()


def test_require_gitlab_token_returns_token(monkeypatch):
    monkeypatch.setenv("GITLAB_TOKEN", "glpat-abc123")
    config = Config()
    assert config.require_gitlab_token() == "glpat-abc123"


def test_require_anthropic_key_returns_key(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "sk-ant-abc123")
    config = Config()
    assert config.require_anthropic_key() == "sk-ant-abc123"


def test_default_constants():
    assert DEFAULT_MODEL == "claude-sonnet-4-20250514"
    assert DEFAULT_FOCUS == ["bugs", "style", "best-practices"]


def test_provider_defaults_to_anthropic(monkeypatch):
    monkeypatch.delenv("MR_REVIEWER_PROVIDER", raising=False)
    config = Config()
    assert config.provider == "anthropic"


def test_provider_from_env(monkeypatch):
    monkeypatch.setenv("MR_REVIEWER_PROVIDER", "ollama")
    config = Config()
    assert config.provider == "ollama"


def test_gemini_api_key_from_env(monkeypatch):
    monkeypatch.setenv("GEMINI_API_KEY", "test-gemini-key")
    config = Config()
    assert config.gemini_api_key == "test-gemini-key"


def test_require_gemini_key_raises_when_empty(monkeypatch):
    monkeypatch.delenv("GEMINI_API_KEY", raising=False)
    config = Config()
    with pytest.raises(ConfigurationError, match="GEMINI_API_KEY"):
        config.require_gemini_key()


def test_require_gemini_key_returns_key(monkeypatch):
    monkeypatch.setenv("GEMINI_API_KEY", "gemini-abc123")
    config = Config()
    assert config.require_gemini_key() == "gemini-abc123"


def test_ollama_host_from_env(monkeypatch):
    monkeypatch.setenv("OLLAMA_HOST", "http://myhost:11434")
    config = Config()
    assert config.ollama_host == "http://myhost:11434"


def test_ollama_host_default(monkeypatch):
    monkeypatch.delenv("OLLAMA_HOST", raising=False)
    config = Config()
    assert config.ollama_host == "http://localhost:11434"


def test_max_comments_default(monkeypatch):
    monkeypatch.delenv("MR_REVIEWER_MAX_COMMENTS", raising=False)
    config = Config()
    assert config.max_comments == 10


def test_max_comments_from_env(monkeypatch):
    monkeypatch.setenv("MR_REVIEWER_MAX_COMMENTS", "5")
    config = Config()
    assert config.max_comments == 5
