import re
from urllib.parse import urlparse

from mr_reviewer.models import MRInfo

GITLAB_MR_PATTERN = re.compile(
    r"^/(?P<namespace>.+)/(?P<project>[^/]+)/-/merge_requests/(?P<iid>\d+)/?$"
)

GITHUB_PR_PATTERN = re.compile(
    r"^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/pull/(?P<number>\d+)/?$"
)


def parse_gitlab_mr_url(url: str) -> MRInfo:
    """Parse a GitLab MR URL into its components.

    Supports gitlab.com and self-hosted instances.
    Handles nested namespaces (groups/subgroups).
    Detects GitLab via the `/-/merge_requests/` path segment.
    """
    parsed = urlparse(url)

    if not parsed.hostname:
        raise ValueError(f"Invalid URL (no hostname): {url}")

    if "/-/merge_requests/" not in parsed.path:
        raise ValueError(
            f"Not a GitLab MR URL (missing /-/merge_requests/ path): {url}"
        )

    match = GITLAB_MR_PATTERN.match(parsed.path)
    if not match:
        raise ValueError(f"Could not parse GitLab MR URL: {url}")

    return MRInfo(
        platform="gitlab",
        host=parsed.hostname,
        namespace=match.group("namespace"),
        project=match.group("project"),
        iid=int(match.group("iid")),
    )


def parse_github_pr_url(url: str) -> MRInfo:
    """Parse a GitHub PR URL into its components.

    Supports github.com URLs of the form:
    https://github.com/{owner}/{repo}/pull/{number}
    """
    parsed = urlparse(url)

    if not parsed.hostname:
        raise ValueError(f"Invalid URL (no hostname): {url}")

    if parsed.hostname != "github.com":
        raise ValueError(f"Not a GitHub PR URL (hostname is not github.com): {url}")

    match = GITHUB_PR_PATTERN.match(parsed.path)
    if not match:
        raise ValueError(f"Could not parse GitHub PR URL: {url}")

    return MRInfo(
        platform="github",
        host=parsed.hostname,
        namespace=match.group("owner"),
        project=match.group("repo"),
        iid=int(match.group("number")),
    )


def parse_mr_url(url: str) -> MRInfo:
    """Auto-detect platform and parse a merge/pull request URL.

    Tries GitHub first (check github.com hostname), then GitLab
    (check /-/merge_requests/ path segment).

    Raises ValueError if neither platform matches.
    """
    parsed = urlparse(url)

    if not parsed.hostname:
        raise ValueError(f"Invalid URL (no hostname): {url}")

    # Try GitHub first
    if parsed.hostname == "github.com":
        return parse_github_pr_url(url)

    # Try GitLab
    # We check the path rather than the hostname to support self-hosted GitLab instances
    if "/-/merge_requests/" in parsed.path:
        return parse_gitlab_mr_url(url)

    raise ValueError(
        f"Unsupported URL: {url}\n"
        "Expected a GitHub PR URL (https://github.com/owner/repo/pull/N) "
        "or a GitLab MR URL (https://host/group/project/-/merge_requests/N)"
    )
