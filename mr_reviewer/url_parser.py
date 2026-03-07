import re
from urllib.parse import urlparse

from mr_reviewer.models import MRInfo

GITLAB_MR_PATTERN = re.compile(
    r"^/(?P<namespace>.+)/(?P<project>[^/]+)/-/merge_requests/(?P<iid>\d+)/?$"
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
