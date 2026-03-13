"""GitLab platform client — implements PlatformClient Protocol."""

import logging
from typing import Any

import gitlab

from mr_reviewer.exceptions import PlatformError
from mr_reviewer.models import (
    DiffFile,
    DiffLine,
    FetchResult,
    GitLabDiffRefs,
    MRInfo,
    MRMetadata,
    ReviewResult,
)

logger = logging.getLogger(__name__)

SEVERITY_PREFIX = {
    "error": "**Error:**",
    "warning": "**Warning:**",
    "info": "**Suggestion:**",
}


class GitLabClient:
    """Client for interacting with the GitLab API.

    Satisfies the PlatformClient Protocol. Caches diff_refs and diff_files
    internally after fetch_mr_changes() for use in post_review().
    """

    def __init__(self, token: str, host: str = "https://gitlab.com") -> None:
        self.gl = gitlab.Gitlab(url=host, private_token=token)
        self.gl.auth()
        self._diff_refs: GitLabDiffRefs | None = None
        self._diff_files: list[DiffFile] = []
        self._project_cache: dict[str, Any] = {}
        self._mr_cache: dict[str, Any] = {}

    def _get_project_and_mr(self, mr_info: MRInfo) -> tuple[Any, Any]:
        """Get project and MR objects, caching after first lookup."""
        cache_key = f"{mr_info.namespace}/{mr_info.project}/{mr_info.iid}"
        if cache_key in self._project_cache:
            return self._project_cache[cache_key], self._mr_cache[cache_key]

        project_path = f"{mr_info.namespace}/{mr_info.project}"
        project = self.gl.projects.get(project_path)
        mr = project.mergerequests.get(mr_info.iid)
        self._project_cache[cache_key] = project
        self._mr_cache[cache_key] = mr
        return project, mr

    def fetch_mr_changes(self, mr_info: MRInfo) -> FetchResult:
        """Fetch MR diff data and metadata.

        Caches diff_refs internally for use by post_review().
        Returns FetchResult with diff_files and metadata.
        """
        _, mr = self._get_project_and_mr(mr_info)
        changes = mr.changes()

        self._diff_refs = GitLabDiffRefs(
            base_sha=changes["diff_refs"]["base_sha"],
            start_sha=changes["diff_refs"]["start_sha"],
            head_sha=changes["diff_refs"]["head_sha"],
        )

        diff_files = []
        for change in changes["changes"]:
            diff_files.append(
                DiffFile(
                    old_path=change["old_path"],
                    new_path=change["new_path"],
                    diff=change.get("diff", ""),
                    new_file=change.get("new_file", False),
                    renamed_file=change.get("renamed_file", False),
                    deleted_file=change.get("deleted_file", False),
                )
            )

        self._diff_files = diff_files

        metadata = MRMetadata(
            title=changes.get("title", ""),
            description=changes.get("description", ""),
            source_branch=changes.get("source_branch", ""),
            target_branch=changes.get("target_branch", ""),
            web_url=changes.get("web_url", ""),
        )

        return FetchResult(diff_files=diff_files, metadata=metadata)

    def fetch_file_content(
        self, mr_info: MRInfo, file_path: str, ref: str
    ) -> str | None:
        """Fetch the full content of a file at a specific ref (branch/sha)."""
        project, _ = self._get_project_and_mr(mr_info)
        try:
            f = project.files.get(file_path=file_path, ref=ref)
            return f.decode().decode("utf-8", errors="replace")
        except gitlab.exceptions.GitlabGetError:
            logger.warning("Could not fetch file %s at ref %s", file_path, ref)
            return None

    def post_review(
        self, mr_info: MRInfo, review: ReviewResult, diff_lines: list[DiffLine]
    ) -> None:
        """Post review to GitLab using internally cached refs.

        Posts inline discussion comments first, then a summary note.
        Raises PlatformError if called before fetch_mr_changes().
        """
        if self._diff_refs is None:
            raise PlatformError(
                "Cannot post review: fetch_mr_changes() must be called first "
                "to cache diff refs."
            )

        _, mr = self._get_project_and_mr(mr_info)

        # Post inline comments
        for comment in review.comments:
            old_path, new_path = self._find_file_paths(comment.file)

            prefix = SEVERITY_PREFIX.get(comment.severity, "")
            body = f"{prefix} {comment.body}" if prefix else comment.body

            position: dict[str, Any] = {
                "position_type": "text",
                "base_sha": self._diff_refs.base_sha,
                "start_sha": self._diff_refs.start_sha,
                "head_sha": self._diff_refs.head_sha,
                "old_path": old_path,
                "new_path": new_path,
            }

            if comment.is_new_line:
                position["new_line"] = comment.line
            else:
                position["old_line"] = comment.line

            try:
                mr.discussions.create({"body": body, "position": position})
                logger.info(
                    "Posted inline comment on %s:%d", comment.file, comment.line
                )
            except gitlab.exceptions.GitlabCreateError as e:
                logger.error(
                    "Failed to post inline comment on %s:%d: %s",
                    comment.file,
                    comment.line,
                    e,
                )

        # Post summary note
        summary_body = f"## AI Code Review\n\n{review.summary}"
        if review.comments:
            error_count = sum(1 for c in review.comments if c.severity == "error")
            warning_count = sum(1 for c in review.comments if c.severity == "warning")
            info_count = sum(1 for c in review.comments if c.severity == "info")
            summary_body += (
                f"\n\n**{len(review.comments)} inline comments posted:** "
                f"{error_count} errors, {warning_count} warnings, {info_count} suggestions"
            )

        mr.notes.create({"body": summary_body})
        logger.info("Review posted successfully!")

    def _find_file_paths(self, comment_file: str) -> tuple[str, str]:
        """Find the old_path and new_path for a file in the cached diff."""
        for df in self._diff_files:
            if df.new_path == comment_file or df.old_path == comment_file:
                return df.old_path, df.new_path
        return comment_file, comment_file
