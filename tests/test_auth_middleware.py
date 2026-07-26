"""Tests for optional Electron loopback API authentication."""

import pytest
from fastapi.testclient import TestClient

from mr_reviewer.api.app import create_app


@pytest.fixture
def authenticated_client(monkeypatch):
    monkeypatch.setenv("MR_REVIEWER_TOKEN", "test-token")
    return TestClient(create_app())


def test_health_does_not_require_electron_token(authenticated_client):
    assert authenticated_client.get("/api/health").status_code == 200


def test_api_rejects_missing_or_invalid_electron_token(authenticated_client):
    assert authenticated_client.get("/api/config/defaults").status_code == 403
    assert authenticated_client.get(
        "/api/config/defaults", headers={"Authorization": "Bearer wrong-token"}
    ).status_code == 403


def test_api_accepts_electron_token(authenticated_client):
    response = authenticated_client.get(
        "/api/config/defaults", headers={"Authorization": "Bearer test-token"}
    )
    assert response.status_code == 200


def test_options_preflight_is_allowed_with_electron_token(authenticated_client):
    response = authenticated_client.options(
        "/api/config/defaults",
        headers={
            "Origin": "null",
            "Access-Control-Request-Method": "GET",
        },
    )
    assert response.status_code == 200
    assert response.headers["access-control-allow-origin"] == "null"
