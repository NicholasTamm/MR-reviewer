"""FastAPI application factory for MR Reviewer."""

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from mr_reviewer.api.routes import router


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
