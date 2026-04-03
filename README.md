# MR Reviewer

MR Reviewer is an AI-powered code reviewer capable of seamlessly interacting with both **GitLab** Merge Requests and **GitHub** Pull Requests. 

By analyzing the MR/PR diffs alongside the fully resolved file contents, the app identifies structural issues, suggests style optimizations, catches bugs, and automatically posts inline comments and a holistic summary note back to the platform. 

It supports working strictly as a command-line utility (fully automated for CI/CD environments) or running as a modern web application for interactive human-in-the-loop code review verifications.

---

## 🛠️ Architecture & Design

The application is split across decoupled architectural layers:

* **Backend Engine (Python & FastAPI)**
  * Handles deep repository extraction via GitLab/GitHub REST API integrations.
  * Extensible Prompt & AI capabilities supporting Anthropic (`claude-3-5-sonnet`), Google (`gemini-1.5-pro`), and Ollama.
  * Uses intelligent diff-parsing boundaries and includes a `parallel` review mode for breaking apart massive pull requests across distributed agents simultaneously.
* **Web UI (React, Vite, Tailwind CSS)**
  * Clean, interactive interface for approving or rejecting the AI's proposed code comments before they hit the origin server.
* **Containers (Docker & Compose)**
  * A unified `docker-compose.yml` wraps the frontend UI on port `3000` and the web server backend on port `8080` for a true one-click reproducible deployment.

---

## 🚀 Setup & Installation

**Prerequisites:** You must have Docker and Docker Compose installed for the easiest setup. Alternatively, you'll need Python 3.11+ and Node.js 20+.

### 1. Environment Configuration
Clone the repository and copy the example environment file:
```bash
cp .env.example .env
```
Inside your new `.env`, configure the required secret keys:
```properties
GITLAB_TOKEN=glpat-...         # To review GitLab Merge Requests
GITHUB_TOKEN=ghp_...           # To review GitHub Pull Requests
ANTHROPIC_API_KEY=sk-ant-...   # Default AI Provider
```

### 2. Running via Docker Compose (Recommended)
Automatically builds the images and spans both the React interface and Python API:
```bash
docker-compose up -d
```
Head to **[http://localhost:3000](http://localhost:3000)** in your browser!

### 3. Local Development (Optional)
If you wish to run the tools natively to modify the source code:
```bash
# Python Backend
pip install -e ".[all]"
python -m mr_reviewer --serve --host 0.0.0.0 --port 8080

# React Frontend (In a separate terminal)
cd frontend
npm install
npm run dev
```

---

## 💻 CLI Usage

You can use the application entirely via the command line interface without ever interacting with the UI.

```bash
# Analyze and post comments to a GitLab MR
python -m mr_reviewer https://gitlab.com/group/project/-/merge_requests/1

# Dry run — print the AI comments to stdout WITHOUT posting
python -m mr_reviewer https://github.com/owner/repository/pull/1 --dry-run

# Specify a custom model and provider
python -m mr_reviewer <URL> --provider gemini --model gemini-1.5-pro

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
| `--model` | Specifically mandate the underlying AI model ID |
| `--parallel` | Enable parallel chunking review mode (splits large diffs) |
| `--parallel-threshold` | Minimum number of changed files required to trigger parallel mode |
| `--max-comments` | Ceil the volume of minor/nit-pick inline comments (defaults to 10) |
| `-v`, `--verbose` | Output extensive DEBUG logging data |
