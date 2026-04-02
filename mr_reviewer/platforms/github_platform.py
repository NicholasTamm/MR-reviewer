"""GitHub platform client — implements PlatformClient Protocol."""

import logging

import httpx

from mr_reviewer.exceptions import PlatformError
from mr_reviewer.models import (
    DiffFile,
    DiffLine,
    FetchResult,
    MRInfo,
    MRMetadata,
    ReviewResult,
)

logger = logging.getLogger(__name__)

MAX_FILES = 3000


class GitHubClient:
    """Client for interacting with the GitHub API.

    Satisfies the PlatformClient Protocol. Caches head_sha
    internally after fetch_mr_changes() for use in post_review().
    """

    def __init__(self, token: str) -> None:
        self._client = httpx.Client(
            base_url="https://api.github.com",
            headers={
                "Authorization": f"Bearer {token}",
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
            },
            timeout=30.0,
        )
        self._head_sha: str | None = None

    def fetch_mr_changes(self, mr_info: MRInfo) -> FetchResult:
        """Fetch PR diff data and metadata.

        Caches head_sha internally for use by post_review().
        Handles pagination for files endpoint (up to MAX_FILES).
        """
        owner = mr_info.namespace
        repo = mr_info.project
        number = mr_info.iid

        # Fetch PR metadata
        pr_resp = self._client.get(f"/repos/{owner}/{repo}/pulls/{number}")
        if pr_resp.status_code != 200:
            raise PlatformError(
                f"Failed to fetch PR #{number}: {pr_resp.status_code} {pr_resp.text}"
            )
        pr_data = pr_resp.json()

        self._head_sha = pr_data["head"]["sha"]

        metadata = MRMetadata(
            title=pr_data.get("title", ""),
            description=pr_data.get("body", "") or "",
            source_branch=pr_data.get("head", {}).get("ref", ""),
            target_branch=pr_data.get("base", {}).get("ref", ""),
            web_url=pr_data.get("html_url", ""),
        )

        # Fetch PR files with pagination
        all_files: list[dict] = []
        page = 1
        while len(all_files) < MAX_FILES:
            files_resp = self._client.get(
                f"/repos/{owner}/{repo}/pulls/{number}/files",
                params={"per_page": 100, "page": page},
            )
            if files_resp.status_code != 200:
                raise PlatformError(
                    f"Failed to fetch PR files: {files_resp.status_code} {files_resp.text}"
                )
            batch = files_resp.json()
            if not batch:
                break
            all_files.extend(batch)
            if len(batch) < 100:
                break
            page += 1

        if len(all_files) >= MAX_FILES:
            logger.warning(
                "PR #%d has %d+ files — capped at %d",
                number,
                len(all_files),
                MAX_FILES,
            )

        diff_files = []
        for f in all_files:
            status = f.get("status", "")
            diff_files.append(
                DiffFile(
                    old_path=f.get("previous_filename", f["filename"]),
                    new_path=f["filename"],
                    diff=f.get("patch", ""),
                    new_file=status == "added",
                    renamed_file=status == "renamed",
                    deleted_file=status == "removed",
                )
            )

        return FetchResult(diff_files=diff_files, metadata=metadata)

    def fetch_file_content(
        self, mr_info: MRInfo, file_path: str, ref: str
    ) -> str | None:
        """Fetch the full content of a file at a specific ref."""
        owner = mr_info.namespace
        repo = mr_info.project

        resp = self._client.get(
            f"/repos/{owner}/{repo}/contents/{file_path}",
            params={"ref": ref},
            headers={"Accept": "application/vnd.github.raw+json"},
        )
        if resp.status_code == 404:
            return None
        if resp.status_code != 200:
            logger.warning(
                "Could not fetch file %s at ref %s: %d",
                file_path,
                ref,
                resp.status_code,
            )
            return None
        return resp.text

    def post_review(
        self, mr_info: MRInfo, review: ReviewResult, diff_lines: list[DiffLine]
    ) -> None:
        """Post an atomic review to GitHub using the pull request reviews API.

        Uses internally cached head_sha from fetch_mr_changes().
        Raises PlatformError if called before fetch_mr_changes().
        """
        if self._head_sha is None:
            raise PlatformError(
                "Cannot post review: fetch_mr_changes() must be called first "
                "to cache head SHA."
            )

        owner = mr_info.namespace
        repo = mr_info.project
        number = mr_info.iid

        # Build inline comments
        comments = []
        for c in review.comments:
            comment_payload: dict = {
                "path": c.file,
                "line": c.line,
                "body": c.body,
                "side": "RIGHT" if c.is_new_line else "LEFT",
            }
            comments.append(comment_payload)

        # Build review payload
        payload: dict = {
            "commit_id": self._head_sha,
            "body": review.summary,
            "event": "COMMENT",
            "comments": comments,
        }

        resp = self._client.post(
            f"/repos/{owner}/{repo}/pulls/{number}/reviews",
            json=payload,
        )
        if resp.status_code not in (200, 201):
            raise PlatformError(
                f"Failed to post review: {resp.status_code} {resp.text}"
            )
        logger.info("Review posted successfully!")
