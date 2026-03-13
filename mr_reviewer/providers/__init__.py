"""AI review provider abstractions."""

from __future__ import annotations

from typing import TYPE_CHECKING

from mr_reviewer.exceptions import ConfigurationError
from mr_reviewer.providers.base import ReviewProvider

if TYPE_CHECKING:
    from mr_reviewer.config import Config

__all__ = ["ReviewProvider", "create_provider"]


def create_provider(config: Config) -> ReviewProvider:
    """Create an AI review provider based on the given configuration.

    Raises ConfigurationError for unknown provider names.
    """
    match config.provider:
        case "anthropic":
            from mr_reviewer.providers.anthropic_provider import AnthropicProvider

            api_key = config.require_anthropic_key()
            return AnthropicProvider(api_key=api_key, model=config.model)

        case "gemini":
            from mr_reviewer.providers.gemini_provider import GeminiProvider

            api_key = config.require_gemini_key()
            return GeminiProvider(api_key=api_key, model=config.model)

        case "ollama":
            from mr_reviewer.providers.ollama_provider import OllamaProvider

            return OllamaProvider(model=config.model, host=config.ollama_host)

        case _:
            raise ConfigurationError(
                f"Unknown provider: {config.provider!r}. "
                f"Supported providers: anthropic, gemini, ollama"
            )
