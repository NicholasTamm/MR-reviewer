import pytest

from mr_reviewer.url_parser import parse_github_pr_url, parse_gitlab_mr_url, parse_mr_url


# --- GitLab URL parsing (existing) ---


def test_valid_gitlab_com_simple_namespace():
    result = parse_gitlab_mr_url("https://gitlab.com/mygroup/myproject/-/merge_requests/42")
    assert result.platform == "gitlab"
    assert result.host == "gitlab.com"
    assert result.namespace == "mygroup"
    assert result.project == "myproject"
    assert result.iid == 42


def test_valid_gitlab_com_nested_namespace():
    result = parse_gitlab_mr_url(
        "https://gitlab.com/group/subgroup/myproject/-/merge_requests/7"
    )
    assert result.namespace == "group/subgroup"
    assert result.project == "myproject"
    assert result.iid == 7


def test_valid_self_hosted_gitlab():
    result = parse_gitlab_mr_url(
        "https://git.example.com/team/repo/-/merge_requests/100"
    )
    assert result.host == "git.example.com"
    assert result.namespace == "team"
    assert result.project == "repo"
    assert result.iid == 100


def test_invalid_url_no_hostname():
    with pytest.raises(ValueError, match="no hostname"):
        parse_gitlab_mr_url("not-a-url")


def test_invalid_url_not_merge_request_path():
    with pytest.raises(ValueError, match="Not a GitLab MR URL"):
        parse_gitlab_mr_url("https://gitlab.com/group/project/-/issues/1")


def test_url_with_trailing_slash():
    result = parse_gitlab_mr_url(
        "https://gitlab.com/mygroup/myproject/-/merge_requests/5/"
    )
    assert result.iid == 5
    assert result.project == "myproject"


# --- GitHub URL parsing ---


def test_parse_github_pr_url_valid():
    result = parse_github_pr_url("https://github.com/owner/repo/pull/42")
    assert result.platform == "github"
    assert result.host == "github.com"
    assert result.namespace == "owner"
    assert result.project == "repo"
    assert result.iid == 42


def test_parse_github_pr_url_trailing_slash():
    result = parse_github_pr_url("https://github.com/owner/repo/pull/99/")
    assert result.iid == 99


def test_parse_github_pr_url_invalid_hostname():
    with pytest.raises(ValueError, match="not github.com"):
        parse_github_pr_url("https://gitlab.com/owner/repo/pull/1")


def test_parse_github_pr_url_invalid_path():
    with pytest.raises(ValueError, match="Could not parse"):
        parse_github_pr_url("https://github.com/owner/repo/issues/1")


def test_parse_github_pr_url_no_hostname():
    with pytest.raises(ValueError, match="no hostname"):
        parse_github_pr_url("not-a-url")


# --- Auto-detection ---


def test_parse_mr_url_detects_github():
    result = parse_mr_url("https://github.com/owner/repo/pull/42")
    assert result.platform == "github"
    assert result.namespace == "owner"
    assert result.project == "repo"
    assert result.iid == 42


def test_parse_mr_url_detects_gitlab():
    result = parse_mr_url("https://gitlab.com/group/project/-/merge_requests/1")
    assert result.platform == "gitlab"
    assert result.namespace == "group"
    assert result.project == "project"
    assert result.iid == 1


def test_parse_mr_url_unsupported():
    with pytest.raises(ValueError, match="Unsupported URL"):
        parse_mr_url("https://bitbucket.org/owner/repo/pull-requests/1")
