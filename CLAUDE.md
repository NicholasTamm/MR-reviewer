# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build the Go binary (TUI, headless review, Electron serve)
go build -o mr-reviewer ./cmd/mr-reviewer

# TUI
./mr-reviewer
./mr-reviewer --config

# Headless review
./mr-reviewer review <URL> --provider echo --dry-run
./mr-reviewer review <URL> --provider anthropic --post

# Electron local API (loopback only; MR_REVIEWER_TOKEN required)
MR_REVIEWER_TOKEN=dev ./mr-reviewer serve --host 127.0.0.1 --port 8080

# Product verification
go test ./...
cd frontend && npm ci && npm run build

# Electron dev
cd frontend && npm ci && npm run electron:dev
```

## Environment Variables

Platform:
- `GITLAB_TOKEN` — GitLab PAT with `api` scope
- `GITHUB_TOKEN` — GitHub PAT
- `GITLAB_OAUTH_CLIENT_ID` / `GITHUB_OAUTH_CLIENT_ID` — public OAuth client IDs

Provider (whichever you use):
- `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `XAI_API_KEY`, `GEMINI_API_KEY` / `GOOGLE_API_KEY`, `KIMI_API_KEY`, `DEEPSEEK_API_KEY`

Optional:
- `MR_REVIEWER_MODEL` — model id
- `MR_REVIEWER_FOCUS` — comma-separated focus areas (default: `bugs,style,best-practices`)
- `MR_REVIEWER_PROVIDER` — `anthropic`, `openai`, `xai`, `google`, `kimi`, `deepseek`, `echo`, or `ollama`
- `OLLAMA_HOST` — Ollama server URL (default: `http://localhost:11434`)
- `MR_REVIEWER_PARALLEL` / `MR_REVIEWER_PARALLEL_THRESHOLD`
- `MR_REVIEWER_MAX_COMMENTS` — non-critical inline comment budget
- `MR_REVIEWER_TOKEN` — required by `serve`; Electron generates this
- `MR_REVIEWER_HOME` — override `~/.mr-reviewer`

Copy `.env.example` to `.env` for local Electron unpackaged runs.

## Architecture

`cmd/mr-reviewer` is the composition root. It starts the TUI, `review`, `serve`, or `auth`.

**Review flow (`internal/review`):**
1. Parse the MR/PR URL
2. Fetch diff + file contents via `internal/platform` (GitLab / GitHub)
3. Build prompts and call `internal/provider`
4. Optional parallel split/merge
5. Drop off-diff comments and enforce the comment budget
6. Dry-run JSON or post via the platform client

**HTTP API (`internal/api`):** Electron-compatible `/api/*` served by `mr-reviewer serve` on `127.0.0.1` only.

**Providers (`internal/provider`):** anthropic, openai, xai, google (`gemini` alias), kimi, deepseek, echo, ollama, plus `providers.jsonc` slugs.

**Auth / config:** `~/.mr-reviewer/auth.json`, `config.json`, optional `providers.jsonc`.

## Frontend

React 19, Vite, TypeScript, Tailwind CSS v4, Electron.

Unpackaged Electron runs `go run ./cmd/mr-reviewer serve`. Packaged builds use `scripts/build-backend.sh` (`go build` → `frontend/resources/backend/mr-reviewer-server`).

Do not reintroduce a Python package, FastAPI server, or PyInstaller backend.
