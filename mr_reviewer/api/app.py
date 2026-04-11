"""FastAPI application factory for MR Reviewer."""

import os

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.base import BaseHTTPMiddleware
from starlette.requests import Request
from starlette.responses import JSONResponse

from mr_reviewer.api.routes import router


class BearerAuthMiddleware(BaseHTTPMiddleware):
    def __init__(self, app, token: str) -> None:
        super().__init__(app)
        self._token = token

    async def dispatch(self, request: Request, call_next):  # type: ignore[override]
        # Skip auth for health check (used by Electron to wait for backend)
        if request.url.path == '/api/health':
            return await call_next(request)
        auth = request.headers.get('Authorization', '')
        if auth != f'Bearer {self._token}':
            return JSONResponse({'detail': 'Unauthorized'}, status_code=403)
        return await call_next(request)


def create_app() -> FastAPI:
    """Create and configure the FastAPI application."""
    app = FastAPI(
        title="MR Reviewer",
        description="AI-powered merge request reviewer with human-in-the-loop review",
        version="0.1.0",
    )

    # CORS for frontend dev server and production
    app.add_middleware(
        CORSMiddleware,
        allow_origins=[
            "http://localhost:5173",
            "http://localhost:3000",
            "http://localhost:8080",
            "file://",
        ],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # Auth middleware: add AFTER CORS so it runs first (Starlette applies in reverse order)
    _token = os.environ.get('MR_REVIEWER_TOKEN')
    if _token:
        app.add_middleware(BearerAuthMiddleware, token=_token)

    app.include_router(router)

    @app.get("/api/health")
    async def health() -> dict[str, str]:
        return {"status": "ok"}

    return app
