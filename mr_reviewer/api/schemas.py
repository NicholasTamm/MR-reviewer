"""Request/response schemas for the MR Reviewer API."""

from datetime import datetime

from pydantic import BaseModel


class ReviewRequest(BaseModel):
    """Request to submit a new review."""

    url: str
    provider: str = "anthropic"
    model: str | None = None
    focus: list[str] = ["bugs", "style", "best-practices"]
    max_comments: int = 10
    parallel: bool = False
    auto_post: bool = False


class JobStatus(BaseModel):
    """Status of a review job."""

    job_id: str
    status: str  # "pending", "fetching", "reviewing", "complete", "failed", "posted"
    progress: str | None = None
    created_at: datetime
    url: str


class MRMetadataResponse(BaseModel):
    """MR/PR metadata in API responses."""

    title: str = ""
    description: str = ""
    source_branch: str = ""
    target_branch: str = ""
    web_url: str = ""


class CommentDetail(BaseModel):
    """A single review comment with context for the frontend."""

    id: str
    file: str
    line: int
    body: str
    severity: str
    is_new_line: bool
    diff_context: list[str]
    approved: bool = True


class ReviewResponse(BaseModel):
    """Full review results."""

    job_id: str
    summary: str
    comments: list[CommentDetail]
    metadata: MRMetadataResponse


class PostRequest(BaseModel):
    """Request to post approved comments."""

    comment_ids: list[str]
    summary: str


class CommentEditRequest(BaseModel):
    """Request to edit a comment body."""

    body: str


class ConfigDefaults(BaseModel):
    """Current default configuration values."""

    provider: str
    model: str
    focus: list[str]
    max_comments: int
    parallel: bool
    parallel_threshold: int
