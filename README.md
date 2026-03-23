# MR Reviewer

AI-powered GitLab merge request reviewer using Claude. Fetches MR diffs, sends them to Claude for analysis, and posts inline comments + a summary note back to the MR.

## Setup

1. Install:
   ```bash
   pip install -e .
   ```

2. Set environment variables:
   ```bash
   export GITLAB_TOKEN=glpat-xxxxxxxxxxxxxxxxxxxx    # GitLab PAT with 'api' scope
   export ANTHROPIC_API_KEY=sk-ant-xxxxxxxxxxxxxxxxxxxx
   ```

## Usage

```bash
# Review an MR (posts comments to GitLab)
python -m mr_reviewer https://gitlab.com/group/project/-/merge_requests/1

# Dry run — print review to stdout without posting
python -m mr_reviewer https://gitlab.com/group/project/-/merge_requests/1 --dry-run

# Custom focus areas
python -m mr_reviewer <URL> --focus "security,performance,bugs"

# Use a different Claude model
python -m mr_reviewer <URL> --model claude-opus-4-20250514

# Verbose logging
python -m mr_reviewer <URL> --dry-run -v
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--dry-run` | off | Print review to stdout instead of posting |
| `--focus` | `bugs,style,best-practices` | Comma-separated review focus areas |
| `--model` | `claude-sonnet-4-20250514` | Claude model to use |
| `-v, --verbose` | off | Enable debug logging |

## How It Works

1. Parses the GitLab MR URL (supports gitlab.com and self-hosted)
2. Fetches the MR diff and full contents of changed files via GitLab API
3. Sends the diff + file contents to Claude with configurable review focus
4. Claude returns structured output: summary + inline comments with severity
5. Validates that comment line numbers exist in the actual diff
6. Posts inline discussion comments first, then a summary note
