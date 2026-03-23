from unittest.mock import MagicMock, patch

import pytest

from mr_reviewer.config import Config
from mr_reviewer.exceptions import ConfigurationError
from mr_reviewer.models import ReviewResult
from mr_reviewer.providers import ReviewProvider, create_provider
from mr_reviewer.providers.anthropic_provider import AnthropicProvider


class TestReviewProviderProtocol:
    """Test that providers satisfy the ReviewProvider protocol."""

    def test_anthropic_provider_is_review_provider(self):
        with patch("anthropic.Anthropic"):
            provider = AnthropicProvider(api_key="test-key", model="test-model")
        assert isinstance(provider, ReviewProvider)

    def test_gemini_provider_is_review_provider(self):
        with patch.dict("sys.modules", {"google": MagicMock(), "google.genai": MagicMock()}):
            from mr_reviewer.providers.gemini_provider import GeminiProvider

            provider = GeminiProvider(api_key="test-key", model="test-model")
            assert isinstance(provider, ReviewProvider)

    def test_ollama_provider_is_review_provider(self):
        with patch.dict("sys.modules", {"ollama": MagicMock()}):
            from mr_reviewer.providers.ollama_provider import OllamaProvider

            provider = OllamaProvider(model="test-model")
            assert isinstance(provider, ReviewProvider)


class TestAnthropicProvider:
    """Test AnthropicProvider.run_review() with mocked client."""

    def test_run_review_returns_review_result(self):
        # Create a mock tool_use block
        mock_tool_block = MagicMock()
        mock_tool_block.type = "tool_use"
        mock_tool_block.input = {
            "summary": "Looks good overall.",
            "comments": [
                {
                    "file": "src/main.py",
                    "line": 10,
                    "body": "Consider adding a docstring.",
                    "severity": "info",
                },
            ],
        }

        mock_response = MagicMock()
        mock_response.content = [mock_tool_block]

        with patch("anthropic.Anthropic") as MockAnthropic:
            mock_client = MockAnthropic.return_value
            mock_client.messages.create.return_value = mock_response

            provider = AnthropicProvider(api_key="test-key", model="test-model")
            result = provider.run_review("system prompt", "user message")

        assert isinstance(result, ReviewResult)
        assert result.summary == "Looks good overall."
        assert len(result.comments) == 1
        assert result.comments[0].file == "src/main.py"
        assert result.comments[0].line == 10
        assert result.comments[0].severity == "info"

    def test_run_review_no_tool_block(self):
        mock_response = MagicMock()
        mock_response.content = []

        with patch("anthropic.Anthropic") as MockAnthropic:
            mock_client = MockAnthropic.return_value
            mock_client.messages.create.return_value = mock_response

            provider = AnthropicProvider(api_key="test-key", model="test-model")
            result = provider.run_review("system prompt", "user message")

        assert isinstance(result, ReviewResult)
        assert "Error" in result.summary
        assert result.comments == []


class TestCreateProvider:
    """Test the create_provider factory function."""

    def test_creates_anthropic_provider(self, monkeypatch):
        monkeypatch.setenv("ANTHROPIC_API_KEY", "test-key")
        monkeypatch.delenv("MR_REVIEWER_PROVIDER", raising=False)
        monkeypatch.delenv("MR_REVIEWER_MODEL", raising=False)

        with patch("anthropic.Anthropic"):
            config = Config()
            config.provider = "anthropic"
            provider = create_provider(config)

        assert isinstance(provider, AnthropicProvider)

    def test_creates_gemini_provider(self, monkeypatch):
        monkeypatch.setenv("GEMINI_API_KEY", "test-key")

        with patch.dict("sys.modules", {"google": MagicMock(), "google.genai": MagicMock()}):
            from mr_reviewer.providers.gemini_provider import GeminiProvider

            config = Config()
            config.provider = "gemini"
            provider = create_provider(config)

            assert isinstance(provider, GeminiProvider)

    def test_creates_ollama_provider(self, monkeypatch):
        with patch.dict("sys.modules", {"ollama": MagicMock()}):
            from mr_reviewer.providers.ollama_provider import OllamaProvider

            config = Config()
            config.provider = "ollama"
            provider = create_provider(config)

            assert isinstance(provider, OllamaProvider)

    def test_unknown_provider_raises_configuration_error(self):
        config = Config()
        config.provider = "unknown-provider"

        with pytest.raises(ConfigurationError, match="Unknown provider"):
            create_provider(config)
