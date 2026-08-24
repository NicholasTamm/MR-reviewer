# MR Reviewer

MR Reviewer is an AI-powered code reviewer for **GitLab** Merge Requests and **GitHub** Pull Requests (including GitHub Enterprise and self-hosted GitLab).

It reads the diff and changed-file contents, asks a model for a structured review, then lets you approve, reject, or edit comments before posting them.

The primary interface is a **Go / Bubble Tea TUI**. There is also a headless CLI for agents and CI, and an **Electron** desktop app that talks to the same Go server.

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

1. **Platform access** — GitLab for the dashboard, plus GitHub if you review PRs:
    ```bash
    ./mr-reviewer auth login gitlab --client-id YOUR_APPLICATION_ID # browser PKCE
    ./mr-reviewer auth login gitlab --device --client-id YOUR_APPLICATION_ID # headless
    ./mr-reviewer auth login gitlab --api-key       # PAT fallback, hidden TTY prompt
    ./mr-reviewer auth login github --client-id YOUR_CLIENT_ID # device OAuth
    ./mr-reviewer auth login github --api-key       # PAT fallback
    ./mr-reviewer auth login gitlab --api-key glpat-...   # non-interactive
    ./mr-reviewer auth status
    ```
    GitLab OAuth is GitLab.com-only. Create your own application at
    `https://gitlab.com/-/user_settings/applications`, configure exactly
    `http://127.0.0.1:8620/oauth/callback` as the redirect URI, select the
    `api` scope, and supply its **Application ID** with `--client-id` or
    `GITLAB_OAUTH_CLIENT_ID`. Do not provide the application secret: the
    browser flow uses PKCE and does not need or store it. The TUI reads
    `GITLAB_OAUTH_CLIENT_ID` when you choose GitLab login. Device authorization
    requires a GitLab instance with the documented device grant available;
    use browser PKCE if GitLab rejects the device-code request.

    GitHub OAuth is GitHub.com-only. Register your own OAuth App at
    `https://github.com/settings/developers`, enable **Device Flow**, and pass
    its public **Client ID** with `--client-id` or `GITHUB_OAUTH_CLIENT_ID`.
    The device flow requests only GitHub's classic `repo` scope, which is the
    OAuth App scope required to review private pull requests and post review
    comments. Do not provide or store the app client secret. GitHub may return
    an expiring access/refresh-token pair when that optional OAuth App feature
    is enabled; MR Reviewer refreshes only such device-flow credentials using
    their stored client ID. GitHub Enterprise Server login is not supported.

    Or export `GITLAB_TOKEN` / `GITHUB_TOKEN`. Environment PATs win over stored
    credentials and remain restricted to GitLab.com / GitHub.com respectively.

2. **Model provider** — same contract as strike: `anthropic`, `openai`, `xai`, `google` (`gemini` alias), `kimi`, `deepseek`, or `echo` (offline):
   ```bash
   ./mr-reviewer auth login anthropic --api-key    # paste a key
   ./mr-reviewer auth login openai                 # Sign in with ChatGPT (localhost:1455)
   ./mr-reviewer auth login openai --api-key       # paste an OpenAI key instead
   ./mr-reviewer auth login xai                    # browser OAuth (127.0.0.1:56121)
   ./mr-reviewer auth login xai --device           # headless / SSH
   ./mr-reviewer auth status
   ./mr-reviewer auth logout gitlab
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
| Auth | `j`/`k` · `enter` login · `k` platform PAT · `d` xAI/GitLab/GitHub device · `x` logout · `esc` back. OAuth progress blocks this screen until it succeeds, fails, or is canceled (`c`/`esc`); `r` retries a failed flow. |
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

## Electron

The desktop app is the React renderer in `frontend/`. Electron starts `mr-reviewer serve` on `127.0.0.1` and talks to it over local HTTP.

```bash
go build -o mr-reviewer ./cmd/mr-reviewer   # same binary the TUI uses
cd frontend
npm ci
npm run electron:dev                        # Vite + Electron; unpackaged spawn is `go run ./cmd/mr-reviewer serve`
npm run build                               # frontend verification build
```

Packaged builds compile the Go server with `scripts/build-backend.sh` (`npm run build:backend` / `npm run dist:mac`).

`serve` binds `127.0.0.1` only and requires `MR_REVIEWER_TOKEN`. Credentials and settings come from `~/.mr-reviewer`.

---

## Architecture

* **Go binary** (`cmd/mr-reviewer`) — TUI, headless `review`, and loopback `serve` for Electron. Shared review engine, GitLab/GitHub clients, and model providers live under `internal/`.
* **Electron frontend** (`frontend/`) — React 19 + Vite + Tailwind. Approves, rejects, and edits comments before they are posted.
* **Go + Electron only** — reviews run through the Go binary; the desktop UI is the Electron renderer.

---

## Verify

```bash
go test ./...
cd frontend && npm ci && npm run build
```
