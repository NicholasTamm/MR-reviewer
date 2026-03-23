import logging

from mr_reviewer.exceptions import ProviderError
from mr_reviewer.models import ReviewResult

logger = logging.getLogger(__name__)


class OllamaProvider:
    """AI review provider using a local Ollama instance."""

    def __init__(
        self, model: str = "llama3.2", host: str = "http://localhost:11434"
    ) -> None:
        try:
            import ollama  # noqa: F401
        except ImportError:
            raise ProviderError(
                "The ollama package is required for the Ollama provider.\n"
                "Install it with: pip install 'mr-reviewer[ollama]'"
            )

        self._model = model
        self._host = host

    def run_review(self, system_prompt: str, user_message: str) -> ReviewResult:
        """Send the diff and context to Ollama and get a structured review back."""
        import ollama

        logger.info("Sending review request to Ollama (%s)...", self._model)

        try:
            response = ollama.chat(
                model=self._model,
                messages=[
                    {"role": "system", "content": system_prompt},
                    {"role": "user", "content": user_message},
                ],
                format=ReviewResult.model_json_schema(),
                options={"temperature": 0},
            )
        except Exception as e:
            raise ProviderError(f"Ollama API error: {e}") from e

        try:
            return ReviewResult.model_validate_json(response.message.content)
        except Exception as e:
            raise ProviderError(
                f"Failed to parse Ollama response as ReviewResult: {e}"
            ) from e
