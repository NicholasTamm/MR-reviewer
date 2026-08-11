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
    GitLabMergeRequestSummary,
    GitLabProjectMergeRequests,
    GitLabProjectSummary,
    MRInfo,
    MRMetadata,
    ReviewResult,
)

logger = logging.getLogger(__name__)

class GitLabClient:
    """Client for interacting with the GitLab API.

    Satisfies the PlatformClient Protocol. Caches diff_refs and diff_files
    internally after fetch_mr_changes() for use in post_review().
    """

    def __init__(self, token: str, host: str = "https://gitlab.com") -> None:
        self.gl = gitlab.Gitlab(url=host, private_token=token)
        try:
            self.gl.auth()
        except gitlab.exceptions.GitlabAuthenticationError as e:
            status = getattr(e, "response_code", "unknown")
            raise PlatformError(
                f"GitLab authentication failed at {host} (HTTP {status}). "
                "Check that GITLAB_TOKEN is valid, has 'api' scope, and belongs to this GitLab instance. "
                "If the token was just changed in .env, recreate the backend container."
            ) from e
        self._diff_refs: GitLabDiffRefs | None = None
        self._diff_files: list[DiffFile] = []
        self._project_cache: dict[str, Any] = {}
        self._mr_cache: dict[str, Any] = {}

    def list_visible_merge_requests(self, search: str = "") -> list[GitLabProjectMergeRequests]:
        """List accessible open merge requests, grouped by project path."""
        try:
            merge_requests = self.gl.mergerequests.list(
                scope="all", state="opened", order_by="updated_at", sort="desc",
                per_page=100, get_all=True,
            )
        except gitlab.exceptions.GitlabListError as e:
            raise PlatformError("GitLab API error listing merge requests") from e

        groups: dict[int, GitLabProjectMergeRequests] = {}
        query = search.casefold().strip()
        for mr in merge_requests:
            data = mr.asdict() if hasattr(mr, "asdict") else mr.__dict__
            reference = data.get("references", {}).get("full", "")
            project_path = reference.rsplit("!", 1)[0] or str(data["project_id"])
            title = data.get("title", "")
            if query and query not in project_path.casefold() and query not in title.casefold():
                continue
            project_id = int(data["project_id"])
            group = groups.setdefault(
                project_id,
                GitLabProjectMergeRequests(project_id=project_id, project_path=project_path, merge_requests=[]),
            )
            group.merge_requests.append(
                GitLabMergeRequestSummary(
                    project_id=project_id, project_path=project_path, iid=data["iid"], title=title,
                    author=data.get("author", {}).get("name", ""),
                    source_branch=data.get("source_branch", ""), target_branch=data.get("target_branch", ""),
                    updated_at=data.get("updated_at", ""), web_url=data.get("web_url", ""),
                    draft=data.get("draft", False),
                )
            )
        return sorted(groups.values(), key=lambda group: group.project_path.casefold())

    def list_visible_projects(self, search: str = "") -> list[GitLabProjectSummary]:
        """List projects accessible to the token without traversing their MRs."""
        try:
            projects = self.gl.projects.list(
                membership=True, archived=False, simple=True, order_by="path",
                sort="asc", per_page=100, get_all=True,
            )
        except gitlab.exceptions.GitlabListError as e:
            raise PlatformError("GitLab API error listing projects") from e
        query = search.casefold().strip()
        return [
            GitLabProjectSummary(project_id=project.id, project_path=project.path_with_namespace, web_url=getattr(project, "web_url", ""))
            for project in projects
            if not query or query in project.path_with_namespace.casefold()
        ]

    def list_project_merge_requests(self, project_id: int) -> GitLabProjectMergeRequests:
        try:
            project = self.gl.projects.get(project_id)
            merge_requests = project.mergerequests.list(
                state="opened", order_by="updated_at", sort="desc", per_page=100, get_all=True,
            )
        except gitlab.exceptions.GitlabGetError as e:
            raise PlatformError("GitLab project not found or inaccessible") from e
        except gitlab.exceptions.GitlabListError as e:
            raise PlatformError("GitLab API error listing project merge requests") from e
        return GitLabProjectMergeRequests(
            project_id=project_id, project_path=project.path_with_namespace,
            merge_requests=[GitLabMergeRequestSummary(
                project_id=project_id, project_path=project.path_with_namespace, iid=mr.iid,
                title=mr.title, author=getattr(mr.author, "name", "") if hasattr(mr, "author") else "",
                source_branch=mr.source_branch, target_branch=mr.target_branch,
                updated_at=mr.updated_at, web_url=mr.web_url, draft=getattr(mr, "draft", False),
            ) for mr in merge_requests],
        )

    def _get_project_and_mr(self, mr_info: MRInfo) -> tuple[Any, Any]:
        """Get project and MR objects, caching after first lookup."""
        cache_key = f"{mr_info.namespace}/{mr_info.project}/{mr_info.iid}"
        if cache_key in self._project_cache:
            return self._project_cache[cache_key], self._mr_cache[cache_key]

        project_path = f"{mr_info.namespace}/{mr_info.project}"
        try:
            project = self.gl.projects.get(project_path)
        except gitlab.exceptions.GitlabGetError as e:
            if e.response_code == 404:
                raise PlatformError(f"GitLab project not found: {project_path}") from e
            raise PlatformError(f"GitLab API error: {e.error_message}") from e
        try:
            mr = project.mergerequests.get(mr_info.iid)
        except gitlab.exceptions.GitlabGetError as e:
            if e.response_code == 404:
                raise PlatformError(
                    "Merge request not found — check the URL and that your token has access"
                ) from e
            raise PlatformError(f"GitLab API error: {e.error_message}") from e
        self._project_cache[cache_key] = project
        self._mr_cache[cache_key] = mr
        return project, mr

    def fetch_mr_changes(self, mr_info: MRInfo) -> FetchResult:
        """Fetch MR diff data and metadata.

        Caches diff_refs internally for use by post_review().
        Returns FetchResult with diff_files and metadata.
        """
        _, mr = self._get_project_and_mr(mr_info)
        try:
            changes = mr.changes()
        except gitlab.exceptions.GitlabGetError as e:
            raise PlatformError(f"GitLab API error fetching MR changes: {e.error_message}") from e

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
                mr.discussions.create({"body": comment.body, "position": position})
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
