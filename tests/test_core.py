from unittest.mock import MagicMock, patch

import pytest

from mr_reviewer.core import (
    _enforce_comment_threshold,
    build_unified_diff as _build_unified_diff,
    review_mr,
)
from mr_reviewer.models import (
    DiffFile,
    FetchResult,
    MRMetadata,
    ReviewComment,
    ReviewResult,
)


class TestBuildUnifiedDiff:
    """Test _build_unified_diff() for various file scenarios."""

    def test_normal_modified_file(self):
        diff_files = [
            DiffFile(
                old_path="src/main.py",
                new_path="src/main.py",
                diff="@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
            )
        ]
        result = _build_unified_diff(diff_files)
        assert "--- a/src/main.py" in result
        assert "+++ b/src/main.py" in result
        assert "+import sys" in result

    def test_renamed_file(self):
        diff_files = [
            DiffFile(
                old_path="old_name.py",
                new_path="new_name.py",
                diff="@@ -1,3 +1,3 @@\n import os\n-old\n+new\n",
                renamed_file=True,
            )
        ]
        result = _build_unified_diff(diff_files)
        assert "--- a/old_name.py" in result
        assert "+++ b/new_name.py" in result

    def test_deleted_file(self):
        diff_files = [
            DiffFile(
                old_path="removed.py",
                new_path="removed.py",
                diff="@@ -1,3 +0,0 @@\n-import os\n-import sys\n-import json\n",
                deleted_file=True,
            )
        ]
        result = _build_unified_diff(diff_files)
        assert "--- a/removed.py" in result
        assert "+++ b/removed.py" in result
        assert "-import os" in result

    def test_new_file(self):
        diff_files = [
            DiffFile(
                old_path="brand_new.py",
                new_path="brand_new.py",
                diff="@@ -0,0 +1,2 @@\n+import os\n+print('hello')\n",
                new_file=True,
            )
        ]
        result = _build_unified_diff(diff_files)
        assert "--- a/brand_new.py" in result
        assert "+++ b/brand_new.py" in result
        assert "+import os" in result

    def test_binary_file_skipped(self):
        diff_files = [
            DiffFile(
                old_path="image.png",
                new_path="image.png",
                diff="",
                is_binary=True,
            )
        ]
        result = _build_unified_diff(diff_files)
        assert result == ""

    def test_empty_diff_skipped(self):
        diff_files = [
            DiffFile(
                old_path="empty.py",
                new_path="empty.py",
                diff="",
            )
        ]
        result = _build_unified_diff(diff_files)
        assert result == ""

    def test_multiple_files(self):
        diff_files = [
            DiffFile(
                old_path="a.py",
                new_path="a.py",
                diff="@@ -1,2 +1,3 @@\n import os\n+import sys\n",
            ),
            DiffFile(
                old_path="b.py",
                new_path="b.py",
                diff="@@ -1,2 +1,3 @@\n import json\n+import csv\n",
            ),
        ]
        result = _build_unified_diff(diff_files)
        assert "--- a/a.py" in result
        assert "--- a/b.py" in result


class TestReviewMR:
    """Test review_mr() orchestration with mocked provider and platform client."""

    def _make_fetch_result(self):
        return FetchResult(
            diff_files=[
                DiffFile(
                    old_path="main.py",
                    new_path="main.py",
                    diff="@@ -1,3 +1,4 @@\n import os\n+import sys\n \n def main():\n",
                )
            ],
            metadata=MRMetadata(
                title="Test MR",
                description="A test",
                source_branch="feature",
                target_branch="main",
                web_url="https://github.com/owner/repo/pull/1",
            ),
        )

    def _make_review_result(self):
        return ReviewResult(
            summary="Looks good.",
            comments=[
                ReviewComment(
                    file="main.py",
                    line=2,
                    body="Consider using a constant.",
                    severity="info",
                )
            ],
        )

    @patch("mr_reviewer.core.Config")
    def test_review_mr_dry_run(self, MockConfig, capsys):
        mock_config = MockConfig.return_value
        mock_config.default_focus = ["bugs"]

        mock_provider = MagicMock()
        mock_provider.run_review.return_value = self._make_review_result()

        mock_platform = MagicMock()
        mock_platform.fetch_mr_changes.return_value = self._make_fetch_result()
        mock_platform.fetch_file_content.return_value = "import os\nimport sys\n\ndef main():\n    pass\n"

        result = review_mr(
            url="https://github.com/owner/repo/pull/1",
            provider=mock_provider,
            platform_client=mock_platform,
            dry_run=True,
        )

        assert isinstance(result, ReviewResult)
        assert result.summary == "Looks good."
        # post_review should NOT be called in dry-run
        mock_platform.post_review.assert_not_called()
        # Should print output
        captured = capsys.readouterr()
        assert "Test MR" in captured.out

    @patch("mr_reviewer.core.Config")
    def test_review_mr_posts_review(self, MockConfig):
        mock_config = MockConfig.return_value
        mock_config.default_focus = ["bugs"]

        mock_provider = MagicMock()
        mock_provider.run_review.return_value = self._make_review_result()

        mock_platform = MagicMock()
        mock_platform.fetch_mr_changes.return_value = self._make_fetch_result()
        mock_platform.fetch_file_content.return_value = "import os\nimport sys\n\ndef main():\n    pass\n"

        result = review_mr(
            url="https://github.com/owner/repo/pull/1",
            provider=mock_provider,
            platform_client=mock_platform,
            dry_run=False,
        )

        assert isinstance(result, ReviewResult)
        # post_review should be called
        mock_platform.post_review.assert_called_once()

    @patch("mr_reviewer.core.Config")
    def test_review_mr_no_changes(self, MockConfig):
        mock_config = MockConfig.return_value
        mock_config.default_focus = ["bugs"]

        mock_provider = MagicMock()
        mock_platform = MagicMock()
        mock_platform.fetch_mr_changes.return_value = FetchResult(
            diff_files=[],
            metadata=MRMetadata(title="Empty MR"),
        )

        result = review_mr(
            url="https://gitlab.com/group/project/-/merge_requests/1",
            provider=mock_provider,
            platform_client=mock_platform,
        )

        assert "No changes" in result.summary
        mock_provider.run_review.assert_not_called()

    @patch("mr_reviewer.core.Config")
    def test_review_mr_sets_is_new_line(self, MockConfig):
        mock_config = MockConfig.return_value
        mock_config.default_focus = ["bugs"]

        # Return a comment on an addition line
        review_result = ReviewResult(
            summary="Found issues.",
            comments=[
                ReviewComment(
                    file="main.py",
                    line=2,
                    body="Unused import.",
                    severity="warning",
                )
            ],
        )

        mock_provider = MagicMock()
        mock_provider.run_review.return_value = review_result

        mock_platform = MagicMock()
        mock_platform.fetch_mr_changes.return_value = self._make_fetch_result()
        mock_platform.fetch_file_content.return_value = "import os\nimport sys\n"

        result = review_mr(
            url="https://github.com/owner/repo/pull/1",
            provider=mock_provider,
            platform_client=mock_platform,
            dry_run=True,
        )

        # The comment should have is_new_line set (True for addition)
        assert len(result.comments) == 1
        assert result.comments[0].is_new_line is True


class TestEnforceCommentThreshold:
    """Test _enforce_comment_threshold() comment budget enforcement."""

    def _make_comment(self, file, line, severity):
        return ReviewComment(
            file=file, line=line, body=f"{severity} issue", severity=severity
        )

    def test_under_budget_no_change(self):
        comments = [
            self._make_comment("a.py", 1, "warning"),
            self._make_comment("a.py", 2, "info"),
        ]
        result = _enforce_comment_threshold(comments, max_comments=5)
        assert len(result) == 2

    def test_threshold_truncates_non_critical(self):
        comments = [
            self._make_comment("a.py", 1, "error"),
            self._make_comment("a.py", 2, "error"),
            self._make_comment("a.py", 3, "warning"),
            self._make_comment("a.py", 4, "warning"),
            self._make_comment("a.py", 5, "info"),
            self._make_comment("a.py", 6, "info"),
            self._make_comment("a.py", 7, "info"),
        ]
        result = _enforce_comment_threshold(comments, max_comments=5)
        # 2 critical + 3 non-critical (budget = 5 - 2 = 3)
        assert len(result) == 5
        error_count = sum(1 for c in result if c.severity == "error")
        assert error_count == 2

    def test_critical_comments_override_threshold(self):
        comments = [self._make_comment("a.py", i, "error") for i in range(8)]
        result = _enforce_comment_threshold(comments, max_comments=3)
        # All 8 error comments kept despite budget of 3
        assert len(result) == 8

    def test_warnings_prioritized_over_info(self):
        comments = [
            self._make_comment("a.py", 1, "info"),
            self._make_comment("a.py", 2, "warning"),
            self._make_comment("a.py", 3, "info"),
            self._make_comment("a.py", 4, "warning"),
        ]
        result = _enforce_comment_threshold(comments, max_comments=2)
        # Budget is 2, no critical. Keep 2 highest priority: warnings
        assert len(result) == 2
        assert all(c.severity == "warning" for c in result)

    def test_sorted_by_file_and_line(self):
        comments = [
            self._make_comment("b.py", 10, "error"),
            self._make_comment("a.py", 5, "warning"),
            self._make_comment("a.py", 1, "error"),
        ]
        result = _enforce_comment_threshold(comments, max_comments=10)
        assert [(c.file, c.line) for c in result] == [
            ("a.py", 1),
            ("a.py", 5),
            ("b.py", 10),
        ]
