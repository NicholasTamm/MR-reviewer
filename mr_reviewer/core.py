import logging

from mr_reviewer.diff_parser import (
    determine_line_type,
    get_changed_file_paths,
    parse_diff,
    validate_comment_line,
)
from mr_reviewer.config import Config
from mr_reviewer.exceptions import ReviewError
from mr_reviewer.models import DiffFile, MRInfo, MRMetadata, ReviewResult
from mr_reviewer.platforms.base import PlatformClient
from mr_reviewer.prompts import build_system_prompt, build_user_message
from mr_reviewer.providers.base import ReviewProvider
from mr_reviewer.url_parser import parse_mr_url

logger = logging.getLogger(__name__)


def build_unified_diff(diff_files: list[DiffFile]) -> str:
    """Combine individual file diffs into a single unified diff string."""
    parts = []
    for df in diff_files:
        if df.is_binary or not df.diff:
            continue
        header = f"--- a/{df.old_path}\n+++ b/{df.new_path}\n"
        parts.append(header + df.diff)
    return "\n".join(parts)


def review_mr(
    url: str,
    provider: ReviewProvider,
    platform_client: PlatformClient,
    focus: list[str] | None = None,
    dry_run: bool = False,
    parallel: bool = False,
    parallel_threshold: int = 10,
) -> ReviewResult:
    """Main review orchestration function.

    This function is platform-agnostic and can be called from CLI,
    API endpoints, or Discord handlers. The caller is responsible for
    constructing the appropriate ReviewProvider and PlatformClient
    and passing them in.
    """
    config = Config()

    # Parse URL
    try:
        mr_info = parse_mr_url(url)
    except ValueError as e:
        raise ReviewError(str(e))

    logger.info(
        "Reviewing MR !%d on %s/%s/%s",
        mr_info.iid,
        mr_info.host,
        mr_info.namespace,
        mr_info.project,
    )

    # Fetch MR data
    logger.info("Fetching MR changes...")
    fetch_result = platform_client.fetch_mr_changes(mr_info)

    if not fetch_result.diff_files:
        logger.warning("No changes found in MR")
        return ReviewResult(summary="No changes found in this MR.", comments=[])

    # Build unified diff
    unified_diff = build_unified_diff(fetch_result.diff_files)
    diff_lines = parse_diff(unified_diff)

    # Fetch full file contents for changed files
    logger.info(
        "Fetching file contents for %d changed files...",
        len(fetch_result.diff_files),
    )
    file_contents: dict[str, str] = {}
    changed_paths = get_changed_file_paths(unified_diff)
    source_ref = fetch_result.metadata.source_branch or mr_info.host

    for path in changed_paths:
        content = platform_client.fetch_file_content(mr_info, path, ref=source_ref)
        if content is not None:
            file_contents[path] = content

    # Build prompts
    focus_areas = focus or config.default_focus
    system_prompt = build_system_prompt(focus_areas)
    user_message = build_user_message(
        title=fetch_result.metadata.title,
        description=fetch_result.metadata.description,
        diff=unified_diff,
        file_contents=file_contents,
    )

    # Call AI provider
    if parallel and len(fetch_result.diff_files) >= parallel_threshold:
        from mr_reviewer.parallel import parallel_review as _parallel_review  # noqa: PLC0415
        review = _parallel_review(
            provider=provider,
            diff_files=fetch_result.diff_files,
            file_contents=file_contents,
            focus_areas=focus_areas,
            metadata=fetch_result.metadata,
            num_agents=2,
        )
    else:
        review = provider.run_review(system_prompt, user_message)

    # Validate comments against diff and set is_new_line
    valid_comments = []
    for comment in review.comments:
        if validate_comment_line(comment.file, comment.line, diff_lines):
            comment_with_line_type = comment.model_copy(
                update={
                    "is_new_line": determine_line_type(
                        comment.file, comment.line, diff_lines
                    )
                }
            )
            valid_comments.append(comment_with_line_type)
        else:
            logger.warning(
                "Dropping comment on %s:%d — line not found in diff",
                comment.file,
                comment.line,
            )
    review = review.model_copy(update={"comments": valid_comments})

    # Output or post
    if dry_run:
        _print_review(review, fetch_result.metadata)
    else:
        platform_client.post_review(mr_info, review, diff_lines)

    return review


def _print_review(review: ReviewResult, metadata: MRMetadata) -> None:
    """Print the review to stdout (dry-run mode)."""
    print(f"\n{'='*60}")
    print(f"Review for: {metadata.title or 'Unknown MR'}")
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
