"""API route handlers for MR Reviewer."""

import asyncio
import logging
import uuid

from fastapi import APIRouter, HTTPException

from mr_reviewer.api.schemas import (
    CommentDetail,
    CommentEditRequest,
    ConfigDefaults,
    JobStatus,
    MRMetadataResponse,
    PostRequest,
    ReviewRequest,
    ReviewResponse,
)
from mr_reviewer.api.state import JobData, job_store
from mr_reviewer.config import Config
from mr_reviewer.core import _enforce_comment_threshold, build_unified_diff
from mr_reviewer.diff_parser import (
    annotate_diff,
    determine_line_type,
    get_changed_file_paths,
    parse_diff,
    validate_comment_line,
)
from mr_reviewer.exceptions import ConfigurationError, MRReviewerError, PlatformError, ProviderError
from mr_reviewer.models import DiffLine, ReviewComment, ReviewResult
from mr_reviewer.platforms import create_platform_client
from mr_reviewer.prompts import build_system_prompt, build_user_message
from mr_reviewer.providers import create_provider
from mr_reviewer.url_parser import parse_mr_url

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api")


def _extract_diff_context(
    file_path: str, line: int, diff_lines: list[DiffLine], context: int = 3
) -> list[str]:
    """Extract surrounding diff lines for a comment's target location."""
    # Find all diff lines for this file, ordered by appearance
    file_lines = [dl for dl in diff_lines if dl.file_path == file_path]
    if not file_lines:
        return []

    # Find the index of the target line
    target_idx = None
    for i, dl in enumerate(file_lines):
        if dl.new_line == line or dl.old_line == line:
            target_idx = i
            break

    if target_idx is None:
        return []

    # Extract context window
    start = max(0, target_idx - context)
    end = min(len(file_lines), target_idx + context + 1)
    result = []
    for dl in file_lines[start:end]:
        prefix = dl.line_type if dl.line_type in ("+", "-") else " "
        result.append(f"{prefix}{dl.content.rstrip()}")
    return result


_AUTH_KEYWORDS = frozenset({"401", "unauthorized", "authentication", "forbidden"})


def _classify_error(e: Exception) -> str:
    """Map an exception to a structured error_type string."""
    if isinstance(e, ConfigurationError):
        return "config"
    if isinstance(e, ValueError):
        return "invalid_url"
    msg = str(e).lower()
    if isinstance(e, PlatformError):
        return "platform_auth" if any(kw in msg for kw in _AUTH_KEYWORDS) else "platform"
    if isinstance(e, ProviderError):
        return "provider_auth" if any(kw in msg for kw in _AUTH_KEYWORDS) else "provider"
    return "unknown"


def _run_review_sync(job_id: str, request: ReviewRequest) -> None:
    """Run the review synchronously (called via asyncio.to_thread)."""
    try:
        config = Config()
        if request.provider:
            config.provider = request.provider
        if request.model:
            config.model = request.model

        # Parse URL
        job_store.update(job_id, status="fetching", progress="Parsing URL...")
        mr_info = parse_mr_url(request.url)

        # Create clients
        platform_client = create_platform_client(config, mr_info)
        provider = create_provider(config)

        # Store references for later posting
        job_store.update(
            job_id,
            mr_info=mr_info,
            platform_client=platform_client,
            progress="Fetching MR changes...",
        )

        # Fetch MR data
        fetch_result = platform_client.fetch_mr_changes(mr_info)

        if not fetch_result.diff_files:
            job_store.update(
                job_id,
                status="complete",
                progress="Complete",
                summary="No changes found in this MR.",
                comments=[],
                metadata=MRMetadataResponse(**fetch_result.metadata.model_dump()),
            )
            return

        # Build unified diff
        unified_diff = build_unified_diff(fetch_result.diff_files)
        diff_lines = parse_diff(unified_diff)

        # Fetch file contents
        job_store.update(
            job_id,
            progress=f"Fetching contents for {len(fetch_result.diff_files)} files...",
        )
        file_contents: dict[str, str] = {}
        changed_paths = get_changed_file_paths(unified_diff)
        source_ref = fetch_result.metadata.source_branch
        if not source_ref:
            logger.warning("No source branch in metadata — file content fetches may fail")
            source_ref = "HEAD"

        for path in changed_paths:
            content = platform_client.fetch_file_content(mr_info, path, ref=source_ref)
            if content is not None:
                file_contents[path] = content

        # Build prompts and run AI review
        job_store.update(job_id, status="reviewing", progress="Running AI review...")
        focus_areas = request.focus
        system_prompt = build_system_prompt(focus_areas, max_comments=request.max_comments)
        user_message = build_user_message(
            title=fetch_result.metadata.title,
            description=fetch_result.metadata.description,
            diff=annotate_diff(unified_diff),
            file_contents=file_contents,
        )

        if request.parallel and len(fetch_result.diff_files) >= config.parallel_threshold:
            from mr_reviewer.parallel import parallel_review as _parallel_review  # noqa: PLC0415

            review = _parallel_review(
                provider=provider,
                diff_files=fetch_result.diff_files,
                file_contents=file_contents,
                focus_areas=focus_areas,
                metadata=fetch_result.metadata,
                num_agents=2,
                max_comments=request.max_comments,
            )
        else:
            review = provider.run_review(system_prompt, user_message)

        # Validate comments against diff
        valid_comments: list[ReviewComment] = []
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

        # Enforce threshold
        valid_comments = _enforce_comment_threshold(valid_comments, request.max_comments)

        # Build CommentDetail list with UUIDs and diff context
        comment_details = [
            CommentDetail(
                id=str(uuid.uuid4()),
                file=c.file,
                line=c.line,
                body=c.body,
                severity=c.severity,
                is_new_line=c.is_new_line,
                diff_context=_extract_diff_context(c.file, c.line, diff_lines),
                approved=True,
            )
            for c in valid_comments
        ]

        metadata_response = MRMetadataResponse(**fetch_result.metadata.model_dump())

        # If auto-post, post immediately
        if request.auto_post:
            job_store.update(job_id, progress="Posting review...")
            filtered_review = ReviewResult(
                summary=review.summary,
                comments=valid_comments,
            )
            platform_client.post_review(mr_info, filtered_review, diff_lines)
            job_store.update(
                job_id,
                status="posted",
                progress="Posted",
                summary=review.summary,
                comments=comment_details,
                metadata=metadata_response,
                diff_lines=diff_lines,
            )
        else:
            job_store.update(
                job_id,
                status="complete",
                progress="Review complete",
                summary=review.summary,
                comments=comment_details,
                metadata=metadata_response,
                diff_lines=diff_lines,
            )

    except Exception as e:
        error_type = _classify_error(e)
        logger.error("Review job %s failed [%s]: %s", job_id, error_type, e)
        job_store.update(
            job_id,
            status="failed",
            progress=None,
            error=str(e),
            error_type=error_type,
        )


@router.post("/reviews", status_code=201)
async def submit_review(request: ReviewRequest) -> JobStatus:
    """Submit a new review request. Returns job ID for polling."""
    job_id = str(uuid.uuid4())
    job = JobData(job_id=job_id, url=request.url)
    job_store.create(job)

    # Run review in background thread
    asyncio.create_task(asyncio.to_thread(_run_review_sync, job_id, request))

    return JobStatus(
        job_id=job_id,
        status="pending",
        progress="Queued",
        created_at=job.created_at,
        url=request.url,
    )


@router.get("/reviews/{job_id}")
async def get_job_status(job_id: str) -> JobStatus:
    """Get the current status of a review job."""
    job = job_store.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="Job not found")

    return JobStatus(
        job_id=job.job_id,
        status=job.status,
        progress=job.progress,
        error=job.error,
        error_type=job.error_type,
        created_at=job.created_at,
        url=job.url,
    )


@router.get("/reviews/{job_id}/results")
async def get_review_results(job_id: str) -> ReviewResponse:
    """Get full review results. Returns 409 if not yet complete."""
    job = job_store.get(job_id)
    if job is None:
        raise HTTPException(status_code=404, detail="Job not found")

    if job.status == "failed":
        raise HTTPException(status_code=500, detail=job.error or "Review failed")

    if job.status not in ("complete", "posted"):
        raise HTTPException(status_code=409, detail=f"Review not ready: {job.status}")

    return ReviewResponse(
        job_id=job.job_id,
        summary=job.summary,
        comments=job.comments,
        metadata=job.metadata,
    )


@router.patch("/reviews/{job_id}/comments/{comment_id}")
async def edit_comment(
    job_id: str, comment_id: str, request: CommentEditRequest
) -> CommentDetail:
    """Edit a comment body text."""
    updated = job_store.edit_comment(job_id, comment_id, request.body)
    if updated is None:
        job = job_store.get(job_id)
        if job is None:
            raise HTTPException(status_code=404, detail="Job not found")
        raise HTTPException(status_code=404, detail="Comment not found")
    return updated


@router.post("/reviews/{job_id}/post")
async def post_review(job_id: str, request: PostRequest) -> JobStatus:
    """Post approved comments to the platform."""
    # Atomically check status and transition to "posting" to prevent double-post
    job = job_store.transition(job_id, from_status="complete", to_status="posting")
    if job is None:
        stored = job_store.get(job_id)
        if stored is None:
            raise HTTPException(status_code=404, detail="Job not found")
        raise HTTPException(
            status_code=409,
            detail=f"Cannot post review in state: {stored.status}",
        )

    if job.platform_client is None or job.mr_info is None:
        job_store.update(job_id, status="complete")  # rollback
        raise HTTPException(status_code=500, detail="Missing platform client state")

    # Filter to approved comments only
    approved_ids = set(request.comment_ids)
    approved_comments = [c for c in job.comments if c.id in approved_ids]

    # Convert CommentDetail back to ReviewComment for posting
    review_comments = [
        ReviewComment(
            file=c.file,
            line=c.line,
            body=c.body,
            severity=c.severity,
            is_new_line=c.is_new_line,
        )
        for c in approved_comments
    ]

    filtered_review = ReviewResult(
        summary=request.summary,
        comments=review_comments,
    )

    try:
        job.platform_client.post_review(job.mr_info, filtered_review, job.diff_lines)
    except MRReviewerError as e:
        raise HTTPException(status_code=502, detail=str(e))

    job_store.update(job_id, status="posted", progress="Posted")

    return JobStatus(
        job_id=job.job_id,
        status="posted",
        progress="Posted",
        created_at=job.created_at,
        url=job.url,
    )


@router.get("/config/defaults")
async def get_config_defaults() -> ConfigDefaults:
    """Return current default configuration values."""
    config = Config()
    return ConfigDefaults(
        provider=config.provider,
        model=config.model,
        focus=config.default_focus,
        max_comments=config.max_comments,
        parallel=config.parallel_review,
        parallel_threshold=config.parallel_threshold,
    )
