import logging
from typing import Any

import gitlab

from mr_reviewer.models import DiffFile, DiffRefs, MRInfo, ReviewComment

logger = logging.getLogger(__name__)


class GitLabClient:
    """Client for interacting with the GitLab API."""

    def __init__(self, token: str, host: str = "https://gitlab.com") -> None:
        self.gl = gitlab.Gitlab(url=host, private_token=token)
        self.gl.auth()

    def _get_project_and_mr(
        self, mr_info: MRInfo
    ) -> tuple[Any, Any]:
        project_path = f"{mr_info.namespace}/{mr_info.project}"
        project = self.gl.projects.get(project_path)
        mr = project.mergerequests.get(mr_info.iid)
        return project, mr

    def fetch_mr_changes(
        self, mr_info: MRInfo
    ) -> tuple[list[DiffFile], DiffRefs, dict[str, Any]]:
        """Fetch MR diff data and metadata.

        Returns:
            - List of DiffFile objects
            - DiffRefs with the three required SHAs
            - MR metadata dict (title, description, source/target branch)
        """
        project, mr = self._get_project_and_mr(mr_info)
        changes = mr.changes()

        diff_refs = DiffRefs(
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

        metadata = {
            "title": changes.get("title", ""),
            "description": changes.get("description", ""),
            "source_branch": changes.get("source_branch", ""),
            "target_branch": changes.get("target_branch", ""),
            "web_url": changes.get("web_url", ""),
        }

        return diff_files, diff_refs, metadata

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

    def post_summary_note(self, mr_info: MRInfo, body: str) -> None:
        """Post a top-level summary comment on the MR."""
        _, mr = self._get_project_and_mr(mr_info)
        mr.notes.create({"body": body})
        logger.info("Posted summary note on MR !%d", mr_info.iid)

    def post_inline_comment(
        self,
        mr_info: MRInfo,
        diff_refs: DiffRefs,
        comment: ReviewComment,
        old_path: str,
        new_path: str,
        is_addition: bool = True,
    ) -> None:
        """Post an inline discussion comment on a specific diff line."""
        _, mr = self._get_project_and_mr(mr_info)

        position: dict[str, Any] = {
            "position_type": "text",
            "base_sha": diff_refs.base_sha,
            "start_sha": diff_refs.start_sha,
            "head_sha": diff_refs.head_sha,
            "old_path": old_path,
            "new_path": new_path,
        }

        if is_addition:
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
