from mr_reviewer.models import DiffFile, GitLabDiffRefs, MRInfo, ReviewComment, ReviewResult


def test_mrinfo_creation():
    info = MRInfo(platform="gitlab", host="gitlab.com", namespace="group", project="repo", iid=1)
    assert info.platform == "gitlab"
    assert info.host == "gitlab.com"
    assert info.namespace == "group"
    assert info.project == "repo"
    assert info.iid == 1


def test_difffile_defaults():
    df = DiffFile(old_path="a.py", new_path="a.py", diff="some diff")
    assert df.new_file is False
    assert df.renamed_file is False
    assert df.deleted_file is False
    assert df.is_binary is False


def test_review_comment_creation():
    comment = ReviewComment(file="main.py", line=10, body="Fix this", severity="error")
    assert comment.file == "main.py"
    assert comment.line == 10
    assert comment.body == "Fix this"
    assert comment.severity == "error"


def test_review_result_empty_comments():
    result = ReviewResult(summary="All good", comments=[])
    assert result.summary == "All good"
    assert result.comments == []


def test_gitlab_diff_refs_creation():
    refs = GitLabDiffRefs(base_sha="aaa", start_sha="bbb", head_sha="ccc")
    assert refs.base_sha == "aaa"
    assert refs.start_sha == "bbb"
    assert refs.head_sha == "ccc"
