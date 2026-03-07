from pydantic import BaseModel


class MRInfo(BaseModel):
    """Parsed merge request URL information."""

    platform: str
    host: str
    namespace: str
    project: str
    iid: int


class DiffFile(BaseModel):
    """A single file's diff information."""

    old_path: str
    new_path: str
    diff: str
    new_file: bool = False
    renamed_file: bool = False
    deleted_file: bool = False
    is_binary: bool = False


class DiffLine(BaseModel):
    """A single changed line in a diff."""

    file_path: str
    old_line: int | None = None
    new_line: int | None = None
    line_type: str  # '+', '-', or ' '
    content: str


class ReviewComment(BaseModel):
    """A single inline review comment."""

    file: str
    line: int
    body: str
    severity: str  # "info", "warning", "error"


class ReviewResult(BaseModel):
    """Complete review output from AI."""

    summary: str
    comments: list[ReviewComment]


class DiffRefs(BaseModel):
    """GitLab diff reference SHAs."""

    base_sha: str
    start_sha: str
    head_sha: str
