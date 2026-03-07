import logging
import sys

from mr_reviewer.claude import run_review
from mr_reviewer.config import Config
from mr_reviewer.diff_parser import (
    get_changed_file_paths,
    parse_diff,
    validate_comment_line,
)
from mr_reviewer.gitlab_client import GitLabClient
from mr_reviewer.models import DiffLine, MRInfo, ReviewResult
from mr_reviewer.prompts import build_system_prompt, build_user_message
from mr_reviewer.url_parser import parse_gitlab_mr_url

logger = logging.getLogger(__name__)


def _build_unified_diff(diff_files: list) -> str:
    """Combine individual file diffs into a single unified diff string."""
    parts = []
    for df in diff_files:
        if df.is_binary or not df.diff:
            continue
        header = f"--- a/{df.old_path}\n+++ b/{df.new_path}\n"
        parts.append(header + df.diff)
    return "\n".join(parts)


def _determine_line_type(
    comment_file: str, comment_line: int, diff_lines: list[DiffLine]
) -> bool:
    """Determine if a comment targets an added line (True) or removed line (False).

    Defaults to addition if ambiguous.
    """
    for dl in diff_lines:
        if dl.file_path == comment_file:
            if dl.new_line == comment_line and dl.line_type == "+":
                return True
            if dl.old_line == comment_line and dl.line_type == "-":
                return False
    return True  # default to addition


def _find_file_paths(
    comment_file: str, diff_files: list
) -> tuple[str, str]:
    """Find the old_path and new_path for a file in the diff."""
    for df in diff_files:
        if df.new_path == comment_file or df.old_path == comment_file:
            return df.old_path, df.new_path
    return comment_file, comment_file


def review_mr(
    url: str,
    focus: list[str] | None = None,
    dry_run: bool = False,
    model: str = "claude-sonnet-4-20250514",
) -> ReviewResult:
    """Main review orchestration function.

    This function is framework-agnostic and can be called from CLI,
    API endpoints, or Discord handlers.
    """
    config = Config()

    # Parse URL
    try:
        mr_info = parse_gitlab_mr_url(url)
    except ValueError as e:
        logger.error("URL parsing failed: %s", e)
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

    logger.info(
        "Reviewing MR !%d on %s/%s/%s",
        mr_info.iid,
        mr_info.host,
        mr_info.namespace,
        mr_info.project,
    )

    # Initialize GitLab client
    gitlab_token = config.require_gitlab_token()
    host_url = f"https://{mr_info.host}"
    gl_client = GitLabClient(token=gitlab_token, host=host_url)

    # Fetch MR data
    logger.info("Fetching MR changes...")
    diff_files, diff_refs, metadata = gl_client.fetch_mr_changes(mr_info)

    if not diff_files:
        logger.warning("No changes found in MR")
        return ReviewResult(summary="No changes found in this MR.", comments=[])

    # Build unified diff
    unified_diff = _build_unified_diff(diff_files)
    diff_lines = parse_diff(unified_diff)

    # Fetch full file contents for changed files
    logger.info("Fetching file contents for %d changed files...", len(diff_files))
    file_contents: dict[str, str] = {}
    changed_paths = get_changed_file_paths(unified_diff)
    source_ref = metadata.get("source_branch", diff_refs.head_sha)

    for path in changed_paths:
        content = gl_client.fetch_file_content(mr_info, path, ref=source_ref)
        if content is not None:
            file_contents[path] = content

    # Build prompts
    focus_areas = focus or config.default_focus
    system_prompt = build_system_prompt(focus_areas)
    user_message = build_user_message(
        title=metadata.get("title", ""),
        description=metadata.get("description", ""),
        diff=unified_diff,
        file_contents=file_contents,
    )

    # Call Claude
    anthropic_key = config.require_anthropic_key()
    review = run_review(
        api_key=anthropic_key,
        system_prompt=system_prompt,
        user_message=user_message,
        model=model,
    )

    # Validate comments against diff
    valid_comments = []
    for comment in review.comments:
        if validate_comment_line(comment.file, comment.line, diff_lines):
            valid_comments.append(comment)
        else:
            logger.warning(
                "Dropping comment on %s:%d — line not found in diff",
                comment.file,
                comment.line,
            )
    review.comments = valid_comments

    # Output or post
    if dry_run:
        _print_review(review, metadata)
    else:
        _post_review(gl_client, mr_info, diff_refs, diff_files, diff_lines, review)

    return review


def _print_review(review: ReviewResult, metadata: dict) -> None:
    """Print the review to stdout (dry-run mode)."""
    print(f"\n{'='*60}")
    print(f"Review for: {metadata.get('title', 'Unknown MR')}")
    print(f"{'='*60}\n")
    print(f"## Summary\n{review.summary}\n")

    if review.comments:
        print(f"## Inline Comments ({len(review.comments)})\n")
        for c in review.comments:
            severity_icon = {"error": "[!]", "warning": "[~]", "info": "[i]"}.get(
                c.severity, "[?]"
            )
            print(f"  {severity_icon} {c.file}:{c.line}")
            print(f"      {c.body}\n")
    else:
        print("No inline comments.\n")


def _post_review(
    gl_client: GitLabClient,
    mr_info: MRInfo,
    diff_refs,
    diff_files: list,
    diff_lines: list[DiffLine],
    review: ReviewResult,
) -> None:
    """Post the review to GitLab."""
    # Post inline comments first (to reduce notification spam)
    for comment in review.comments:
        old_path, new_path = _find_file_paths(comment.file, diff_files)
        is_addition = _determine_line_type(comment.file, comment.line, diff_lines)

        severity_prefix = {
            "error": "**Error:**",
            "warning": "**Warning:**",
            "info": "**Suggestion:**",
        }.get(comment.severity, "")

        prefixed_comment = comment.model_copy(
            update={"body": f"{severity_prefix} {comment.body}"}
        )

        gl_client.post_inline_comment(
            mr_info=mr_info,
            diff_refs=diff_refs,
            comment=prefixed_comment,
            old_path=old_path,
            new_path=new_path,
            is_addition=is_addition,
        )

    # Post summary note last
    summary_body = f"## AI Code Review\n\n{review.summary}"
    if review.comments:
        error_count = sum(1 for c in review.comments if c.severity == "error")
        warning_count = sum(1 for c in review.comments if c.severity == "warning")
        info_count = sum(1 for c in review.comments if c.severity == "info")
        summary_body += (
            f"\n\n**{len(review.comments)} inline comments posted:** "
            f"{error_count} errors, {warning_count} warnings, {info_count} suggestions"
        )

    gl_client.post_summary_note(mr_info, summary_body)
    logger.info("Review posted successfully!")
