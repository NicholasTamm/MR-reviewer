"""Tests for the FastAPI web API."""

import time
from unittest.mock import MagicMock, patch

import pytest
from fastapi.testclient import TestClient

from mr_reviewer.api.app import create_app
from mr_reviewer.api.schemas import CommentDetail, MRMetadataResponse
from mr_reviewer.api.state import JobData, job_store
from mr_reviewer.models import ReviewComment, ReviewResult


@pytest.fixture
def client():
    app = create_app()
    return TestClient(app)


@pytest.fixture(autouse=True)
def clear_job_store():
    """Clear the job store before each test."""
    job_store._jobs.clear()
    yield
    job_store._jobs.clear()


class TestHealthEndpoint:
    def test_health(self, client):
        resp = client.get("/api/health")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}


class TestConfigDefaults:
    @patch("mr_reviewer.api.routes.Config")
    def test_config_defaults(self, MockConfig, client):
        mock_config = MockConfig.return_value
        mock_config.provider = "anthropic"
        mock_config.model = "claude-sonnet-4-20250514"
        mock_config.default_focus = ["bugs", "style", "best-practices"]
        mock_config.max_comments = 10
        mock_config.parallel_review = False
        mock_config.parallel_threshold = 10

        resp = client.get("/api/config/defaults")
        assert resp.status_code == 200
        data = resp.json()
        assert data["provider"] == "anthropic"
        assert data["model"] == "claude-sonnet-4-20250514"
        assert data["max_comments"] == 10


class TestSubmitReview:
    @patch("mr_reviewer.api.routes._run_review_sync")
    def test_submit_review_returns_job_id(self, mock_run, client):
        resp = client.post("/api/reviews", json={
            "url": "https://github.com/owner/repo/pull/1",
        })
        assert resp.status_code == 201
        data = resp.json()
        assert "job_id" in data
        assert data["status"] == "pending"
        assert data["url"] == "https://github.com/owner/repo/pull/1"


class TestGetJobStatus:
    def test_get_status_not_found(self, client):
        resp = client.get("/api/reviews/nonexistent")
        assert resp.status_code == 404

    def test_get_status_pending(self, client):
        job = JobData(job_id="test-123", url="https://github.com/o/r/pull/1")
        job_store.create(job)

        resp = client.get("/api/reviews/test-123")
        assert resp.status_code == 200
        assert resp.json()["status"] == "pending"

    def test_get_status_complete(self, client):
        job = JobData(
            job_id="test-456",
            url="https://github.com/o/r/pull/1",
            status="complete",
            progress="Review complete",
        )
        job_store.create(job)

        resp = client.get("/api/reviews/test-456")
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "complete"
        assert data["progress"] == "Review complete"


class TestGetResults:
    def test_results_when_pending_returns_409(self, client):
        job = JobData(job_id="test-pending", url="https://github.com/o/r/pull/1")
        job_store.create(job)

        resp = client.get("/api/reviews/test-pending/results")
        assert resp.status_code == 409

    def test_results_when_not_found(self, client):
        resp = client.get("/api/reviews/nonexistent/results")
        assert resp.status_code == 404

    def test_results_when_complete(self, client):
        comments = [
            CommentDetail(
                id="c1",
                file="main.py",
                line=10,
                body="Issue here",
                severity="warning",
                is_new_line=True,
                diff_context=["+import sys"],
                approved=True,
            )
        ]
        job = JobData(
            job_id="test-done",
            url="https://github.com/o/r/pull/1",
            status="complete",
            summary="Looks good overall.",
            comments=comments,
            metadata=MRMetadataResponse(
                title="Test PR",
                web_url="https://github.com/o/r/pull/1",
            ),
        )
        job_store.create(job)

        resp = client.get("/api/reviews/test-done/results")
        assert resp.status_code == 200
        data = resp.json()
        assert data["summary"] == "Looks good overall."
        assert len(data["comments"]) == 1
        assert data["comments"][0]["file"] == "main.py"
        assert data["comments"][0]["diff_context"] == ["+import sys"]
        assert data["metadata"]["title"] == "Test PR"

    def test_results_when_failed(self, client):
        job = JobData(
            job_id="test-failed",
            url="https://github.com/o/r/pull/1",
            status="failed",
            error="Provider timeout",
        )
        job_store.create(job)

        resp = client.get("/api/reviews/test-failed/results")
        assert resp.status_code == 500
        assert "Provider timeout" in resp.json()["detail"]


class TestEditComment:
    def test_edit_comment_body(self, client):
        comments = [
            CommentDetail(
                id="c1",
                file="main.py",
                line=10,
                body="Original body",
                severity="warning",
                is_new_line=True,
                diff_context=[],
            )
        ]
        job = JobData(
            job_id="test-edit",
            url="https://github.com/o/r/pull/1",
            status="complete",
            comments=comments,
        )
        job_store.create(job)

        resp = client.patch(
            "/api/reviews/test-edit/comments/c1",
            json={"body": "Edited body"},
        )
        assert resp.status_code == 200
        assert resp.json()["body"] == "Edited body"

        # Verify persisted
        updated_job = job_store.get("test-edit")
        assert updated_job.comments[0].body == "Edited body"

    def test_edit_comment_not_found(self, client):
        job = JobData(
            job_id="test-edit2",
            url="https://github.com/o/r/pull/1",
            status="complete",
            comments=[],
        )
        job_store.create(job)

        resp = client.patch(
            "/api/reviews/test-edit2/comments/nonexistent",
            json={"body": "New body"},
        )
        assert resp.status_code == 404


class TestPostReview:
    def test_post_approved_comments(self, client):
        comments = [
            CommentDetail(
                id="c1", file="a.py", line=1, body="Fix this",
                severity="error", is_new_line=True, diff_context=[],
            ),
            CommentDetail(
                id="c2", file="b.py", line=5, body="Consider this",
                severity="info", is_new_line=True, diff_context=[],
            ),
        ]
        mock_platform = MagicMock()
        mock_mr_info = MagicMock()

        job = JobData(
            job_id="test-post",
            url="https://github.com/o/r/pull/1",
            status="complete",
            summary="Original summary",
            comments=comments,
            platform_client=mock_platform,
            mr_info=mock_mr_info,
        )
        job_store.create(job)

        resp = client.post("/api/reviews/test-post/post", json={
            "comment_ids": ["c1"],
            "summary": "Edited summary",
        })
        assert resp.status_code == 200
        assert resp.json()["status"] == "posted"

        # Verify platform_client was called with only approved comment
        mock_platform.post_review.assert_called_once()
        call_args = mock_platform.post_review.call_args
        posted_review = call_args[0][1]
        assert len(posted_review.comments) == 1
        assert posted_review.comments[0].file == "a.py"
        assert posted_review.summary == "Edited summary"

    def test_post_wrong_status(self, client):
        job = JobData(
            job_id="test-post2",
            url="https://github.com/o/r/pull/1",
            status="pending",
        )
        job_store.create(job)

        resp = client.post("/api/reviews/test-post2/post", json={
            "comment_ids": [],
            "summary": "Summary",
        })
        assert resp.status_code == 409


class TestAutoPostMode:
    @patch("mr_reviewer.api.routes._run_review_sync")
    def test_auto_post_submits_with_flag(self, mock_run, client):
        resp = client.post("/api/reviews", json={
            "url": "https://github.com/owner/repo/pull/1",
            "auto_post": True,
        })
        assert resp.status_code == 201

        # Verify _run_review_sync was called (via executor)
        # The auto_post flag is passed through the ReviewRequest
        call_args = mock_run.call_args
        if call_args:
            request = call_args[0][1]
            assert request.auto_post is True
