from __future__ import annotations

from typing import Protocol, runtime_checkable

from mr_reviewer.models import ReviewResult


@runtime_checkable
class ReviewProvider(Protocol):
    """Protocol that all AI review providers must satisfy."""

    def run_review(self, system_prompt: str, user_message: str) -> ReviewResult: ...
