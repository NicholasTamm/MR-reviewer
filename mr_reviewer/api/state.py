"""In-memory job store for review jobs."""

from __future__ import annotations

import threading
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any

from mr_reviewer.api.schemas import CommentDetail, MRMetadataResponse
from mr_reviewer.models import DiffLine, MRInfo


@dataclass
class JobData:
    """Internal state for a single review job."""

    job_id: str
    url: str
    status: str = "pending"  # pending, fetching, reviewing, complete, failed, posted
    progress: str | None = None
    created_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))
    error: str | None = None

    # Review results (populated when complete)
    summary: str = ""
    comments: list[CommentDetail] = field(default_factory=list)
    metadata: MRMetadataResponse = field(default_factory=MRMetadataResponse)

    # Internal references for posting
    mr_info: MRInfo | None = None
    platform_client: Any = None  # PlatformClient — stored for post step
    diff_lines: list[DiffLine] = field(default_factory=list)


class JobStore:
    """Thread-safe in-memory job store."""

    def __init__(self) -> None:
        self._jobs: dict[str, JobData] = {}
        self._lock = threading.Lock()

    def create(self, job: JobData) -> None:
        with self._lock:
            self._jobs[job.job_id] = job

    def get(self, job_id: str) -> JobData | None:
        with self._lock:
            return self._jobs.get(job_id)

    def update(self, job_id: str, **kwargs: Any) -> None:
        with self._lock:
            job = self._jobs.get(job_id)
            if job is None:
                return
            for key, value in kwargs.items():
                setattr(job, key, value)

    def transition(
        self, job_id: str, from_status: str, to_status: str
    ) -> JobData | None:
        """Atomically transition a job's status. Returns the job if successful, None otherwise."""
        with self._lock:
            job = self._jobs.get(job_id)
            if job is None or job.status != from_status:
                return None
            job.status = to_status
            return job

    def edit_comment(
        self, job_id: str, comment_id: str, body: str
    ) -> CommentDetail | None:
        """Atomically edit a comment's body under the lock."""
        with self._lock:
            job = self._jobs.get(job_id)
            if job is None:
                return None
            for comment in job.comments:
                if comment.id == comment_id:
                    comment.body = body
                    return comment
            return None


# Singleton instance
job_store = JobStore()
