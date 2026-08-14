# MR Reviewer

MR Reviewer is an AI-powered code reviewer for **GitLab** Merge Requests and **GitHub** Pull Requests (including GitHub Enterprise and self-hosted GitLab).

It reads the diff and changed-file contents, asks a model for a structured review, then lets you approve, reject, or edit comments before posting them.

The primary interface is a **Go / Bubble Tea TUI**. There is also a headless CLI for agents and CI. The older Python/React web UI is still in the repo but is not required.

---

## TUI (primary)

### Build and run

```bash
go build -o mr-reviewer ./cmd/mr-reviewer
./mr-reviewer                 # dashboard
./mr-reviewer --config        # settings panel
./mr-reviewer help
```

Requires Go 1.25+ (see `go.mod`).

### First-time setup

1. **Platform token** — GitLab for the dashboard, plus GitHub if you review PRs:
   ```bash
   ./mr-reviewer auth login gitlab    # paste a GITLAB_TOKEN (api scope)
   ./mr-reviewer auth login github    # paste a GITHUB_TOKEN
   ```
   Or export `GITLAB_TOKEN` / `GITHUB_TOKEN`. Env vars win over stored keys.

2. **Model provider** — one of `anthropic`, `openai`, `xai`, `google` (`gemini` alias), `kimi`, `deepseek`, or `echo` (offline):
   ```bash
   ./mr-reviewer auth login anthropic          # API key
   ./mr-reviewer auth login openai             # Sign in with ChatGPT (browser)
   ./mr-reviewer auth login xai                # browser OAuth
   ./mr-reviewer auth login xai --device       # headless / SSH
   ./mr-reviewer auth status
   ```
   Or set `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `XAI_API_KEY`, `GEMINI_API_KEY` / `GOOGLE_API_KEY`, `KIMI_API_KEY`, `DEEPSEEK_API_KEY`.

3. **Hosts (optional)** — GitHub Enterprise or self-hosted GitLab:
   - In the TUI press `c`, or run `./mr-reviewer --config`.
   - Edit **github api** (e.g. `https://ghe.example.com/api/v3`), **gitlab**, and **anthropic** base URL.
   - Press `s` to save. Values land in `~/.mr-reviewer/config.json` (mode `0600`).
   - Env still wins: `MR_REVIEWER_GITHUB_API`, `MR_REVIEWER_GITLAB_URL`, `MR_REVIEWER_ANTHROPIC_URL`.

Credentials and settings live under `~/.mr-reviewer/` (`auth.json`, `config.json`, optional `providers.jsonc`). Override the home with `MR_REVIEWER_HOME`.

### Typical review

1. Start `./mr-reviewer`. Open merge requests visible to your GitLab token appear, grouped by project.
2. `/` search, `j`/`k` move, `enter` to review a listed MR.
3. Or `l` to paste a GitLab MR or GitHub/GHE PR URL.
4. On the link screen: `tab` between URL, provider, model, focus, max comments, and mode. `←`/`→` cycle providers, `enter` or type to edit the model, `space` toggles focus/auto-post, `+/-` changes max comments. `enter` on an empty model field starts typing; `enter` elsewhere runs the review.
5. Wait on the reviewing screen.
6. On the comment list: `j`/`k` move, `a` approve, `r` reject, `e` edit the body, `s` edit the summary, `p` post approved comments + summary.
7. Confirmation: `n` review another, `q` quit.

Mode **review first** (default) always shows the HITL list. **auto-post** posts as soon as the model returns.

### Keyboard map

| Screen | Keys |
|---|---|
| Dashboard | `j`/`k` move · `enter` open MR · `/` search · `l` paste URL · `c` config · `a` auth · `q` quit |
| Link / configure | `tab` fields · `←`/`→` provider · `enter`/type model · `space` toggle · `+/-` max · `enter` run · `esc` back |
| Review (HITL) | `j`/`k` · `a` approve · `r` reject · `e` edit · `s` summary · `p` post · `esc` back |
| Posted | `n` new · `q` quit |
| Auth | `j`/`k` · `enter` login · `x` logout · `d` xAI device · `esc` back |
| Config | `tab`/`j`/`k` fields · `enter`/type edit · `s` save · `esc` back |
| Anywhere | `ctrl+c` quit |

### Custom model providers

Add `~/.mr-reviewer/providers.jsonc` (strike / OpenCode map). Custom slugs show up in the provider list; builtin keys overlay that provider’s HTTP origin:

```jsonc
{
  "acme": {
    "npm": "@ai-sdk/openai-compatible",
    "options": {
      "baseURL": "https://llm.acme.example/v1",
      "apiKey": "{env:ACME_API_KEY}"
    }
  },
  "anthropic": {
    "options": { "baseURL": "https://claude.proxy.example" }
  }
}
```

Credentials stay in env or `auth login` — `options.apiKey` is an env ref only.

### Headless (agents / CI)

```bash
./mr-reviewer review <url> --provider echo --dry-run
./mr-reviewer review <url> --provider anthropic --post
```

JSON on stdout (`summary` + `comments[]` with `file`, `line`, `body`, `severity`). Default is dry-run; `--post` posts. Non-zero exit on failure.

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
