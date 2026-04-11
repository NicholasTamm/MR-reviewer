"""Tests for BearerAuthMiddleware in the FastAPI app."""

import pytest
from fastapi.testclient import TestClient

from mr_reviewer.api.app import create_app


@pytest.fixture
def client_with_token(monkeypatch):
    """TestClient with MR_REVIEWER_TOKEN set to 'testtoken'."""
    monkeypatch.setenv("MR_REVIEWER_TOKEN", "testtoken")
    app = create_app()
    return TestClient(app, raise_server_exceptions=False)


@pytest.fixture
def client_without_token(monkeypatch):
    """TestClient with no MR_REVIEWER_TOKEN set (no auth enforcement)."""
    monkeypatch.delenv("MR_REVIEWER_TOKEN", raising=False)
    app = create_app()
    return TestClient(app, raise_server_exceptions=False)


class TestBearerAuthMiddleware:
    def test_health_passes_without_token_when_auth_enforced(self, client_with_token):
        """Health check must be accessible without Bearer token even when auth is enforced."""
        resp = client_with_token.get("/api/health")
        assert resp.status_code == 200
        assert resp.json() == {"status": "ok"}

    def test_other_route_returns_403_without_token(self, client_with_token):
        """Other routes must return 403 when no Authorization header is provided."""
        resp = client_with_token.get("/api/config/defaults")
        assert resp.status_code == 403
        assert resp.json()["detail"] == "Unauthorized"

    def test_other_route_returns_200_with_correct_token(self, client_with_token):
        """Routes return 200 when correct Bearer token is supplied."""
        resp = client_with_token.get(
            "/api/config/defaults",
            headers={"Authorization": "Bearer testtoken"},
        )
        assert resp.status_code == 200

    def test_other_route_returns_403_with_wrong_token(self, client_with_token):
        """Routes return 403 when an incorrect Bearer token is supplied."""
        resp = client_with_token.get(
            "/api/config/defaults",
            headers={"Authorization": "Bearer wrongtoken"},
        )
        assert resp.status_code == 403
        assert resp.json()["detail"] == "Unauthorized"

    def test_no_auth_enforcement_without_env_var(self, client_without_token):
        """When MR_REVIEWER_TOKEN is not set, routes are accessible without a token."""
        resp = client_without_token.get("/api/health")
        assert resp.status_code == 200

        resp = client_without_token.get("/api/config/defaults")
        assert resp.status_code == 200

    def test_bearer_prefix_required(self, client_with_token):
        """Supplying just the token without 'Bearer ' prefix must be rejected."""
        resp = client_with_token.get(
            "/api/config/defaults",
            headers={"Authorization": "testtoken"},
        )
        assert resp.status_code == 403

    def test_post_route_returns_403_without_token(self, client_with_token):
        """POST routes are also protected."""
        resp = client_with_token.post(
            "/api/reviews",
            json={"url": "https://github.com/owner/repo/pull/1"},
        )
        assert resp.status_code == 403
