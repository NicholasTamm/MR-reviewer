from unittest.mock import MagicMock, patch

import pytest

from mr_reviewer.config import Config
from mr_reviewer.exceptions import ConfigurationError
from mr_reviewer.models import MRInfo
from mr_reviewer.platforms import PlatformClient, create_platform_client


class TestProtocolConformance:
    """Test that platform clients satisfy the PlatformClient protocol."""

    def test_gitlab_client_is_platform_client(self):
        with patch("gitlab.Gitlab") as MockGitlab:
            mock_gl = MockGitlab.return_value
            mock_gl.auth.return_value = None

            from mr_reviewer.platforms.gitlab_platform import GitLabClient

            client = GitLabClient(token="test-token")
            assert isinstance(client, PlatformClient)

    def test_github_client_is_platform_client(self):
        with patch("httpx.Client"):
            from mr_reviewer.platforms.github_platform import GitHubClient

            client = GitHubClient(token="test-token")
            assert isinstance(client, PlatformClient)


class TestCreatePlatformClient:
    """Test the create_platform_client factory function."""

    def test_creates_gitlab_client(self, monkeypatch):
        monkeypatch.setenv("GITLAB_TOKEN", "test-token")
        config = Config()
        mr_info = MRInfo(
            platform="gitlab",
            host="gitlab.com",
            namespace="group",
            project="repo",
            iid=1,
        )

        with patch("gitlab.Gitlab") as MockGitlab:
            mock_gl = MockGitlab.return_value
            mock_gl.auth.return_value = None
            client = create_platform_client(config, mr_info)

        from mr_reviewer.platforms.gitlab_platform import GitLabClient

        assert isinstance(client, GitLabClient)

    def test_creates_github_client(self, monkeypatch):
        monkeypatch.setenv("GITHUB_TOKEN", "test-token")
        config = Config()
        mr_info = MRInfo(
            platform="github",
            host="github.com",
            namespace="owner",
            project="repo",
            iid=42,
        )

        with patch("httpx.Client"):
            client = create_platform_client(config, mr_info)

        from mr_reviewer.platforms.github_platform import GitHubClient

        assert isinstance(client, GitHubClient)

    def test_unknown_platform_raises_configuration_error(self):
        config = Config()
        mr_info = MRInfo(
            platform="bitbucket",
            host="bitbucket.org",
            namespace="owner",
            project="repo",
            iid=1,
        )

        with pytest.raises(ConfigurationError, match="Unknown platform"):
            create_platform_client(config, mr_info)


class TestGitHubClientFetchMRChanges:
    """Test GitHubClient.fetch_mr_changes() with mocked httpx."""

    def _make_client(self, mock_httpx_client):
        with patch("httpx.Client", return_value=mock_httpx_client):
            from mr_reviewer.platforms.github_platform import GitHubClient

            return GitHubClient(token="test-token")

    def _make_mr_info(self):
        return MRInfo(
            platform="github",
            host="github.com",
            namespace="owner",
            project="repo",
            iid=42,
        )

    def test_fetch_mr_changes_basic(self):
        mock_client = MagicMock()

        # PR metadata response
        pr_response = MagicMock()
        pr_response.status_code = 200
        pr_response.json.return_value = {
            "title": "Test PR",
            "body": "PR description",
            "head": {"sha": "abc123", "ref": "feature-branch"},
            "base": {"ref": "main"},
            "html_url": "https://github.com/owner/repo/pull/42",
        }

        # Files response (single page)
        files_response = MagicMock()
        files_response.status_code = 200
        files_response.json.return_value = [
            {
                "filename": "src/main.py",
                "status": "modified",
                "patch": "@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():",
            }
        ]

        mock_client.get.side_effect = [pr_response, files_response]

        client = self._make_client(mock_client)
        mr_info = self._make_mr_info()
        result = client.fetch_mr_changes(mr_info)

        assert result.metadata.title == "Test PR"
        assert result.metadata.description == "PR description"
        assert result.metadata.source_branch == "feature-branch"
        assert result.metadata.target_branch == "main"
        assert len(result.diff_files) == 1
        assert result.diff_files[0].new_path == "src/main.py"
        assert client._head_sha == "abc123"

    def test_fetch_mr_changes_pagination(self):
        mock_client = MagicMock()

        # PR metadata response
        pr_response = MagicMock()
        pr_response.status_code = 200
        pr_response.json.return_value = {
            "title": "Big PR",
            "body": "",
            "head": {"sha": "def456", "ref": "big-change"},
            "base": {"ref": "main"},
            "html_url": "https://github.com/owner/repo/pull/42",
        }

        # Page 1: 100 files (triggers pagination)
        page1_files = [
            {"filename": f"file{i}.py", "status": "modified", "patch": ""}
            for i in range(100)
        ]
        page1_response = MagicMock()
        page1_response.status_code = 200
        page1_response.json.return_value = page1_files

        # Page 2: 50 files (less than 100, stops pagination)
        page2_files = [
            {"filename": f"extra{i}.py", "status": "added", "patch": ""}
            for i in range(50)
        ]
        page2_response = MagicMock()
        page2_response.status_code = 200
        page2_response.json.return_value = page2_files

        mock_client.get.side_effect = [pr_response, page1_response, page2_response]

        client = self._make_client(mock_client)
        mr_info = self._make_mr_info()
        result = client.fetch_mr_changes(mr_info)

        assert len(result.diff_files) == 150
        # Verify 3 GET calls: PR metadata + 2 pages of files
        assert mock_client.get.call_count == 3

    def test_fetch_mr_changes_new_and_deleted_files(self):
        mock_client = MagicMock()

        pr_response = MagicMock()
        pr_response.status_code = 200
        pr_response.json.return_value = {
            "title": "Mixed PR",
            "body": None,
            "head": {"sha": "ghi789", "ref": "mixed"},
            "base": {"ref": "main"},
            "html_url": "https://github.com/owner/repo/pull/42",
        }

        files_response = MagicMock()
        files_response.status_code = 200
        files_response.json.return_value = [
            {"filename": "new_file.py", "status": "added", "patch": "+new content"},
            {"filename": "old_file.py", "status": "removed", "patch": "-old content"},
            {
                "filename": "new_name.py",
                "previous_filename": "old_name.py",
                "status": "renamed",
                "patch": "",
            },
        ]

        mock_client.get.side_effect = [pr_response, files_response]

        client = self._make_client(mock_client)
        mr_info = self._make_mr_info()
        result = client.fetch_mr_changes(mr_info)

        assert len(result.diff_files) == 3
        assert result.diff_files[0].new_file is True
        assert result.diff_files[1].deleted_file is True
        assert result.diff_files[2].renamed_file is True
        assert result.diff_files[2].old_path == "old_name.py"
        assert result.diff_files[2].new_path == "new_name.py"
        # body=None should become empty string
        assert result.metadata.description == ""
