"""Platform client abstractions for VCS integrations."""

from __future__ import annotations

from typing import TYPE_CHECKING

from mr_reviewer.exceptions import ConfigurationError
from mr_reviewer.platforms.base import PlatformClient

if TYPE_CHECKING:
    from mr_reviewer.config import Config
    from mr_reviewer.models import MRInfo

__all__ = ["PlatformClient", "create_platform_client"]


def create_platform_client(config: Config, mr_info: MRInfo) -> PlatformClient:
    """Create a platform client based on the MR info's platform field.

    Raises ConfigurationError for unknown platforms.
    """
    match mr_info.platform:
        case "gitlab":
            from mr_reviewer.platforms.gitlab_platform import GitLabClient

            token = config.require_gitlab_token()
            host_url = f"https://{mr_info.host}"
            return GitLabClient(token=token, host=host_url)

        case "github":
            from mr_reviewer.platforms.github_platform import GitHubClient

            token = config.require_github_token()
            return GitHubClient(token=token)

        case _:
            raise ConfigurationError(
                f"Unknown platform: {mr_info.platform!r}. "
                f"Supported platforms: gitlab, github"
            )
