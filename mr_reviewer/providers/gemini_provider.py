import logging

from mr_reviewer.exceptions import ProviderError
from mr_reviewer.models import ReviewResult

logger = logging.getLogger(__name__)


class GeminiProvider:
    """AI review provider using Google's Gemini API."""

    def __init__(self, api_key: str, model: str = "gemini-2.5-flash") -> None:
        try:
            from google import genai
        except ImportError:
            raise ProviderError(
                "The google-genai package is required for the Gemini provider.\n"
                "Install it with: pip install 'mr-reviewer[gemini]'"
            )

        self._client = genai.Client(api_key=api_key)
        self._model = model

    def run_review(self, system_prompt: str, user_message: str) -> ReviewResult:
        """Send the diff and context to Gemini and get a structured review back."""
        from google.genai import types

        logger.info("Sending review request to Gemini (%s)...", self._model)

        try:
            response = self._client.models.generate_content(
                model=self._model,
                contents=user_message,
                config=types.GenerateContentConfig(
                    system_instruction=system_prompt,
                    response_mime_type="application/json",
                    response_schema=ReviewResult,
                ),
            )
        except Exception as e:
            raise ProviderError(f"Gemini API error: {e}") from e

        try:
            return ReviewResult.model_validate_json(response.text)
        except Exception as e:
            raise ProviderError(
                f"Failed to parse Gemini response as ReviewResult: {e}"
            ) from e
