import logging
from concurrent.futures import ThreadPoolExecutor, as_completed

from mr_reviewer.exceptions import ProviderError
from mr_reviewer.models import DiffFile, MRMetadata, ReviewComment, ReviewResult
from mr_reviewer.diff_parser import annotate_diff
from mr_reviewer.prompts import build_system_prompt, build_user_message
from mr_reviewer.providers.base import ReviewProvider

logger = logging.getLogger(__name__)


def _build_partial_diff(diff_files: list[DiffFile]) -> str:
    """Build a unified diff string from a subset of DiffFiles."""
    from mr_reviewer.core import build_unified_diff  # noqa: PLC0415
    return build_unified_diff(diff_files)


def _merge_results(results: list[ReviewResult]) -> ReviewResult:
    """Merge multiple ReviewResults into one.

    - Concatenates summaries with separator
    - Deduplicates comments by (file, line) — first occurrence wins
    - Sorts merged comments by (file, line)
    """
    if not results:
        return ReviewResult(summary="", comments=[])

    summaries = [r.summary for r in results if r.summary]
    merged_summary = "\n\n---\n\n".join(summaries)

    seen: set[tuple[str, int]] = set()
    merged_comments: list[ReviewComment] = []
    for result in results:
        for comment in result.comments:
            key = (comment.file, comment.line)
            if key not in seen:
                seen.add(key)
                merged_comments.append(comment)

    merged_comments.sort(key=lambda c: (c.file, c.line))

    return ReviewResult(summary=merged_summary, comments=merged_comments)


def parallel_review(
    provider: ReviewProvider,
    diff_files: list[DiffFile],
    file_contents: dict[str, str],
    focus_areas: list[str],
    metadata: MRMetadata,
    num_agents: int = 2,
    max_comments: int = 10,
) -> ReviewResult:
    """Run parallel reviews by partitioning diff_files across num_agents.

    Files are distributed round-robin across agents. Each agent reviews its
    partition concurrently. Results are merged with deduplication.

    If some agents fail, partial results are returned. If all agents fail,
    ProviderError is raised.
    """
    # Partition diff_files round-robin across agents
    partitions: list[list[DiffFile]] = [[] for _ in range(num_agents)]
    for i, df in enumerate(diff_files):
        partitions[i % num_agents].append(df)

    system_prompt = build_system_prompt(focus_areas, max_comments=max_comments)

    def run_agent(partition: list[DiffFile]) -> ReviewResult:
        partial_diff = _build_partial_diff(partition)
        # Only include file_contents for files in this partition
        partition_paths = {df.new_path for df in partition} | {df.old_path for df in partition}
        partial_contents = {
            path: content
            for path, content in file_contents.items()
            if path in partition_paths
        }
        user_message = build_user_message(
            title=metadata.title,
            description=metadata.description,
            diff=annotate_diff(partial_diff),
            file_contents=partial_contents,
        )
        return provider.run_review(system_prompt, user_message)

    results: list[ReviewResult] = []
    futures = {}

    with ThreadPoolExecutor(max_workers=num_agents) as executor:
        for i, partition in enumerate(partitions):
            if not partition:
                continue
            future = executor.submit(run_agent, partition)
            futures[future] = i

        for future in as_completed(futures):
            agent_idx = futures[future]
            try:
                result = future.result()
                results.append(result)
                logger.info("Agent %d completed review successfully", agent_idx)
            except Exception as e:
                logger.error("Agent %d failed: %s", agent_idx, e)

    if not results:
        raise ProviderError("All parallel review agents failed")

    return _merge_results(results)
