"""PlatformClient Protocol — interface for VCS platform integrations."""

from __future__ import annotations

from typing import Protocol, runtime_checkable

from mr_reviewer.models import DiffLine, FetchResult, MRInfo, ReviewResult


@runtime_checkable
class PlatformClient(Protocol):
    """Protocol that all VCS platform clients must satisfy."""

    def fetch_mr_changes(self, mr_info: MRInfo) -> FetchResult:
        """Fetch diff files and metadata. Platform caches internal refs."""
        ...

    def fetch_file_content(self, mr_info: MRInfo, file_path: str, ref: str) -> str | None:
        """Fetch the full content of a file at a specific ref."""
        ...

    def post_review(
        self, mr_info: MRInfo, review: ReviewResult, diff_lines: list[DiffLine]
    ) -> None:
        """Post review using internally cached refs from fetch_mr_changes."""
        ...
