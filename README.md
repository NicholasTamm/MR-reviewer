# MR Reviewer

MR Reviewer is an AI-powered code reviewer capable of seamlessly interacting with both **GitLab** Merge Requests and **GitHub** Pull Requests.

By analyzing the MR/PR diffs alongside the fully resolved file contents, the app identifies structural issues, suggests style optimizations, catches bugs, and automatically posts inline comments and a holistic summary note back to the platform.

It supports three modes:
- **Desktop app (Electron)** — runs entirely locally, credentials stored in the OS keychain, no server required
- **Web app (Docker)** — self-hosted, credentials set via environment variables
- **CLI** — fully automated, ideal for CI/CD pipelines

---

## 🛠️ Architecture & Design

The application is split across decoupled architectural layers:

* **Backend Engine (Python & FastAPI)**
  * Handles deep repository extraction via GitLab/GitHub REST API integrations.
  * Extensible AI capabilities supporting Anthropic (`claude-sonnet-4`), Google (`gemini-2.5-pro`), and Ollama.
  * Parallel review mode splits large PRs across multiple AI agents simultaneously.
* **Desktop App (Electron)**
  * Native desktop window with the full React UI.
  * Credentials stored securely in the OS keychain (macOS Keychain, Windows Credential Manager, libsecret on Linux) — never written to disk or sent over the network.
  * Backend runs as a local subprocess; a shared secret token prevents other processes from accessing the API port.
* **Web UI (React, Vite, Tailwind CSS)**
  * Clean, interactive interface for approving or rejecting AI comments before posting.
* **Containers (Docker & Compose)**
  * `docker-compose.yml` runs the frontend on port `3000` and backend on port `8080`.

---

## 🚀 Setup & Installation

### Option A: Desktop App (Electron) — Recommended for personal use

**Prerequisites:** Python 3.11+, Node.js 20+

```bash
# Install Python backend
pip install -e ".[all]"

# Install frontend dependencies
cd frontend && npm install

# Run the desktop app
npm run electron:dev
```

The app opens a native window. Go to **Settings** to enter your API keys — they are stored in the OS keychain and never leave your machine.

---

### Option B: Web App (Docker)

**Prerequisites:** Docker and Docker Compose

**1. Configure credentials**

```bash
cp .env.example .env
```

Edit `.env` with your keys:
```properties
GITLAB_TOKEN=glpat-...         # To review GitLab Merge Requests
GITHUB_TOKEN=ghp_...           # To review GitHub Pull Requests
ANTHROPIC_API_KEY=sk-ant-...   # Default AI provider
```

**2. Start**

```bash
docker compose up -d
```

Head to **[http://localhost:3000](http://localhost:3000)**

---

### Option C: Local Development (Backend + Frontend separately)

```bash
# Python backend
pip install -e ".[all]"
python -m mr_reviewer --serve --host 0.0.0.0 --port 8080

# React frontend (separate terminal)
cd frontend
npm install
npm run dev
```

Frontend dev server: **[http://localhost:5173](http://localhost:5173)**

---

## 💻 CLI Usage

You can use the application entirely via the command line interface without ever interacting with the UI.

```bash
# Analyze and post comments to a GitLab MR
python -m mr_reviewer https://gitlab.com/group/project/-/merge_requests/1

# Dry run — print the AI comments to stdout WITHOUT posting
python -m mr_reviewer https://github.com/owner/repository/pull/1 --dry-run

# Specify a custom model and provider
python -m mr_reviewer <URL> --provider gemini --model gemini-2.5-pro

# Provide domain-specific focus areas
python -m mr_reviewer <URL> --focus "security,memory-leaks,api-best-practices"

# Enable aggressive parallel processing for massive PRs
python -m mr_reviewer <URL> --parallel --parallel-threshold 10
```

### Advanced CLI Flags

| Flag | Description |
|------|-------------|
| `--serve` | Start the web UI server instead of running a CLI review |
| `--port` / `--host` | Bind settings for the `--serve` daemon |
| `--dry-run` | Print the review output to stdout instead of posting it |
| `--focus` | Comma-separated review focus areas (default: bugs,style,best-practices) |
| `--provider` | AI provider to use (`anthropic`, `gemini`, `ollama`) |
| `--model` | Specify the AI model ID (e.g. `claude-sonnet-4`, `gemini-2.5-pro`) |
| `--parallel` | Enable parallel chunking review mode (splits large diffs) |
| `--parallel-threshold` | Minimum number of changed files required to trigger parallel mode |
| `--max-comments` | Ceil the volume of minor/nit-pick inline comments (defaults to 10) |
| `-v`, `--verbose` | Output extensive DEBUG logging data |
