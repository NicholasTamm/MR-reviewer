from unittest.mock import MagicMock, patch

import pytest

from mr_reviewer.exceptions import ProviderError
from mr_reviewer.models import DiffFile, MRMetadata, ReviewComment, ReviewResult
from mr_reviewer.parallel import _build_partial_diff, _merge_results, parallel_review


def make_diff_file(path: str, diff: str = "@@ -1,1 +1,1 @@\n-old\n+new\n") -> DiffFile:
    return DiffFile(old_path=path, new_path=path, diff=diff)


def make_comment(file: str, line: int, body: str = "test", severity: str = "info") -> ReviewComment:
    return ReviewComment(file=file, line=line, body=body, severity=severity)


class TestMergeResults:
    def test_summaries_concatenated(self):
        r1 = ReviewResult(summary="First summary", comments=[])
        r2 = ReviewResult(summary="Second summary", comments=[])
        merged = _merge_results([r1, r2])
        assert "First summary" in merged.summary
        assert "Second summary" in merged.summary
        assert "\n\n---\n\n" in merged.summary

    def test_comment_deduplication_first_wins(self):
        c1 = make_comment("foo.py", 10, body="first")
        c2 = make_comment("foo.py", 10, body="second")
        r1 = ReviewResult(summary="A", comments=[c1])
        r2 = ReviewResult(summary="B", comments=[c2])
        merged = _merge_results([r1, r2])
        assert len(merged.comments) == 1
        assert merged.comments[0].body == "first"

    def test_comments_sorted_by_file_and_line(self):
        comments_r1 = [make_comment("b.py", 5), make_comment("a.py", 20)]
        comments_r2 = [make_comment("a.py", 3)]
        r1 = ReviewResult(summary="X", comments=comments_r1)
        r2 = ReviewResult(summary="Y", comments=comments_r2)
        merged = _merge_results([r1, r2])
        keys = [(c.file, c.line) for c in merged.comments]
        assert keys == sorted(keys)

    def test_no_duplication_different_lines(self):
        r1 = ReviewResult(summary="A", comments=[make_comment("f.py", 1)])
        r2 = ReviewResult(summary="B", comments=[make_comment("f.py", 2)])
        merged = _merge_results([r1, r2])
        assert len(merged.comments) == 2

    def test_empty_results_returns_empty(self):
        merged = _merge_results([])
        assert merged.summary == ""
        assert merged.comments == []

    def test_single_result_passthrough(self):
        c = make_comment("x.py", 7)
        r = ReviewResult(summary="Only", comments=[c])
        merged = _merge_results([r])
        assert merged.summary == "Only"
        assert len(merged.comments) == 1


class TestBuildPartialDiff:
    def test_builds_diff_for_subset(self):
        files = [
            make_diff_file("a.py", "@@ -1 +1 @@\n-x\n+y\n"),
            make_diff_file("b.py", "@@ -1 +1 @@\n-p\n+q\n"),
        ]
        result = _build_partial_diff(files[:1])
        assert "a.py" in result
        assert "b.py" not in result

    def test_skips_binary_files(self):
        files = [DiffFile(old_path="img.png", new_path="img.png", diff="", is_binary=True)]
        result = _build_partial_diff(files)
        assert result == ""

    def test_empty_list_returns_empty(self):
        assert _build_partial_diff([]) == ""


class TestParallelReview:
    def _make_mock_provider(self, result: ReviewResult):
        provider = MagicMock()
        provider.run_review.return_value = result
        return provider

    def _make_metadata(self) -> MRMetadata:
        return MRMetadata(title="Test PR", description="desc", source_branch="feature", target_branch="main")

    def test_provider_called_num_agents_times(self):
        diff_files = [make_diff_file(f"file{i}.py") for i in range(4)]
        result = ReviewResult(summary="ok", comments=[])
        provider = self._make_mock_provider(result)
        parallel_review(
            provider=provider,
            diff_files=diff_files,
            file_contents={},
            focus_areas=["bugs"],
            metadata=self._make_metadata(),
            num_agents=2,
        )
        assert provider.run_review.call_count == 2

    def test_results_are_merged(self):
        diff_files = [make_diff_file(f"file{i}.py") for i in range(4)]
        c1 = make_comment("file0.py", 1, body="from agent 1")
        c2 = make_comment("file2.py", 5, body="from agent 2")
        results = [
            ReviewResult(summary="Agent 1 summary", comments=[c1]),
            ReviewResult(summary="Agent 2 summary", comments=[c2]),
        ]
        call_count = 0

        def side_effect(system_prompt, user_message):
            nonlocal call_count
            r = results[call_count % len(results)]
            call_count += 1
            return r

        provider = MagicMock()
        provider.run_review.side_effect = side_effect

        merged = parallel_review(
            provider=provider,
            diff_files=diff_files,
            file_contents={},
            focus_areas=["bugs"],
            metadata=self._make_metadata(),
            num_agents=2,
        )
        assert len(merged.comments) == 2

    def test_one_agent_fails_returns_partial(self):
        diff_files = [make_diff_file(f"file{i}.py") for i in range(4)]
        success_result = ReviewResult(summary="ok", comments=[make_comment("file0.py", 1)])
        call_count = 0

        def side_effect(system_prompt, user_message):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise RuntimeError("Agent exploded")
            return success_result

        provider = MagicMock()
        provider.run_review.side_effect = side_effect

        result = parallel_review(
            provider=provider,
            diff_files=diff_files,
            file_contents={},
            focus_areas=["bugs"],
            metadata=self._make_metadata(),
            num_agents=2,
        )
        # Partial result from successful agent should be returned
        assert result.summary == "ok"

    def test_all_agents_fail_raises_provider_error(self):
        diff_files = [make_diff_file(f"file{i}.py") for i in range(4)]
        provider = MagicMock()
        provider.run_review.side_effect = RuntimeError("always fails")

        with pytest.raises(ProviderError, match="All parallel review agents failed"):
            parallel_review(
                provider=provider,
                diff_files=diff_files,
                file_contents={},
                focus_areas=["bugs"],
                metadata=self._make_metadata(),
                num_agents=2,
            )
