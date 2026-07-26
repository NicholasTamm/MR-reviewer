"""FastAPI application factory for MR Reviewer."""

import os
from hmac import compare_digest

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse

from mr_reviewer.api.routes import router


class BearerAuthMiddleware(BaseHTTPMiddleware):
    """Protect the Electron loopback API with its per-launch bearer token."""

    def __init__(self, app, token: str) -> None:
        super().__init__(app)
        self._token = token

    async def dispatch(self, request: Request, call_next):  # type: ignore[override]
        if request.method == "OPTIONS" or request.url.path == "/api/health":
            return await call_next(request)
        expected = f"Bearer {self._token}"
        if not compare_digest(request.headers.get("Authorization", ""), expected):
            return JSONResponse({"detail": "Unauthorized"}, status_code=403)
        return await call_next(request)


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    app = FastAPI(
        title="MR Reviewer",
        description="AI-powered merge request reviewer with human-in-the-loop review",
        version="0.1.0",
    )

    token = os.environ.get("MR_REVIEWER_TOKEN")
    if token:
        app.add_middleware(BearerAuthMiddleware, token=token)

    # CORS must wrap auth so Electron's file-origin preflights succeed.
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[
            "http://localhost:5173",
            "http://localhost:3000",
            "http://localhost:8080",
            "null",
        ],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(router)

    @app.get("/api/health")
    async def health() -> dict[str, str]:
        return {"status": "ok"}

    return app
