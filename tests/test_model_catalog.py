import sys
from types import ModuleType, SimpleNamespace
from unittest.mock import MagicMock, patch

from mr_reviewer.config import Config
from mr_reviewer.providers.model_catalog import (
    _discover_anthropic_models,
    _discover_gemini_models,
    _discover_ollama_models,
)


def test_discovers_anthropic_models(monkeypatch):
    monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")
    with patch("anthropic.Anthropic") as MockAnthropic:
        MockAnthropic.return_value.models.list.return_value.data = [
            SimpleNamespace(id="claude-b"),
            SimpleNamespace(id="claude-a"),
        ]
        result = _discover_anthropic_models(Config())

    assert result.models == ["claude-a", "claude-b"]
    assert result.available is True


def test_discovers_generate_content_gemini_models(monkeypatch):
    monkeypatch.setenv("GEMINI_API_KEY", "test-key")
    genai = MagicMock()
    genai.Client.return_value.models.list.return_value = [
        SimpleNamespace(name="models/gemini-review", supported_actions=["generateContent"]),
        SimpleNamespace(name="models/embedding", supported_actions=["embedContent"]),
    ]
    google = ModuleType("google")
    google.genai = genai
    monkeypatch.setitem(sys.modules, "google", google)

    result = _discover_gemini_models(Config())

    assert result.models == ["models/gemini-review"]
    assert result.available is True


def test_discovers_ollama_models(monkeypatch):
    ollama = MagicMock()
    ollama.Client.return_value.list.return_value.models = [
        SimpleNamespace(model="qwen:latest"),
        SimpleNamespace(model="llama:latest"),
    ]
    monkeypatch.setitem(sys.modules, "ollama", ollama)

    result = _discover_ollama_models(Config())

    assert result.models == ["llama:latest", "qwen:latest"]
    assert result.available is True


def test_missing_provider_key_is_reported_without_request(monkeypatch):
    monkeypatch.delenv("ANTHROPIC_API_KEY", raising=False)

    result = _discover_anthropic_models(Config())

    assert result.available is False
    assert "ANTHROPIC_API_KEY" in (result.error or "")
