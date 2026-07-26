"""Discover review-capable models from configured AI providers."""

from dataclasses import dataclass

from mr_reviewer.config import Config
from mr_reviewer.exceptions import ConfigurationError


@dataclass(frozen=True)
class ProviderModels:
    provider: str
    models: list[str]
    available: bool
    error: str | None = None


def discover_models(config: Config) -> list[ProviderModels]:
    """Return models usable for reviews without exposing credentials to clients."""
    return [
        _discover_anthropic_models(config),
        _discover_gemini_models(config),
        _discover_ollama_models(config),
    ]


def _discover_anthropic_models(config: Config) -> ProviderModels:
    try:
        import anthropic

        client = anthropic.Anthropic(api_key=config.require_anthropic_key())
        models = sorted(model.id for model in client.models.list(limit=100).data)
        return ProviderModels("anthropic", models, True)
    except Exception as e:
        return _unavailable("anthropic", e)


def _discover_gemini_models(config: Config) -> ProviderModels:
    try:
        from google import genai

        client = genai.Client(api_key=config.require_gemini_key())
        models = []
        for model in client.models.list():
            actions = getattr(model, "supported_actions", None) or getattr(
                model, "supported_generation_methods", []
            )
            if any(action.lower() == "generatecontent" for action in actions):
                models.append(model.name)
        return ProviderModels("gemini", sorted(models), True)
    except Exception as e:
        return _unavailable("gemini", e)


def _discover_ollama_models(config: Config) -> ProviderModels:
    try:
        import ollama

        response = ollama.Client(host=config.ollama_host).list()
        models = sorted(model.model for model in response.models)
        return ProviderModels("ollama", models, True)
    except Exception as e:
        return _unavailable("ollama", e)


def _unavailable(provider: str, error: Exception) -> ProviderModels:
    if isinstance(error, ConfigurationError):
        return ProviderModels(provider, [], False, str(error))
    return ProviderModels(provider, [], False, "Unable to retrieve available models.")
