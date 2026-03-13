import logging

from unidiff import PatchSet

from mr_reviewer.models import DiffLine

logger = logging.getLogger(__name__)


def parse_diff(diff_string: str) -> list[DiffLine]:
    """Parse a unified diff string into a list of changed lines.

    Returns only added and removed lines (not context lines),
    with their file paths and line numbers mapped for GitLab's position API.
    """
    lines: list[DiffLine] = []

    try:
        patch = PatchSet(diff_string)
    except Exception as e:
        logger.error("Failed to parse diff: %s", e)
        return lines

    for patched_file in patch:
        if patched_file.is_binary_file:
            logger.info("Skipping binary file: %s", patched_file.path)
            continue

        file_path = patched_file.path
        if patched_file.is_rename:
            file_path = patched_file.target_file
            # Strip leading b/ if present
            if file_path.startswith("b/"):
                file_path = file_path[2:]

        # Strip leading a/ or b/ prefixes from path
        if file_path.startswith(("a/", "b/")):
            file_path = file_path[2:]

        for hunk in patched_file:
            for line in hunk:
                if line.line_type == " ":
                    continue  # skip context lines

                lines.append(
                    DiffLine(
                        file_path=file_path,
                        old_line=line.source_line_no,
                        new_line=line.target_line_no,
                        line_type=line.line_type,
                        content=line.value,
                    )
                )

    return lines


def get_changed_file_paths(diff_string: str) -> list[str]:
    """Extract unique file paths from a diff."""
    paths: list[str] = []
    try:
        patch = PatchSet(diff_string)
    except Exception as e:
        logger.error("Failed to parse diff: %s", e)
        return paths

    for patched_file in patch:
        if patched_file.is_binary_file:
            continue
        path = patched_file.path
        if patched_file.is_rename:
            path = patched_file.target_file
        if path.startswith(("a/", "b/")):
            path = path[2:]
        paths.append(path)

    return paths


def determine_line_type(
    comment_file: str, comment_line: int, diff_lines: list[DiffLine]
) -> bool:
    """Determine if a comment targets an added line (True) or removed line (False).

    Defaults to addition if ambiguous.
    """
    for dl in diff_lines:
        if dl.file_path == comment_file:
            if dl.new_line == comment_line and dl.line_type == "+":
                return True
            if dl.old_line == comment_line and dl.line_type == "-":
                return False
    return True  # default to addition


def validate_comment_line(
    file_path: str, line: int, diff_lines: list[DiffLine]
) -> bool:
    """Check if a (file, line) pair exists in the parsed diff as a changed line."""
    for dl in diff_lines:
        if dl.file_path == file_path:
            # Match against new_line for additions, old_line for deletions
            if dl.new_line == line or dl.old_line == line:
                return True
    return False
