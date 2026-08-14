package review

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	gitlabMRPattern = regexp.MustCompile(`^/(?P<namespace>.+)/(?P<project>[^/]+)/-/merge_requests/(?P<iid>\d+)/?$`)
	githubPRPattern = regexp.MustCompile(`^/(?P<owner>[^/]+)/(?P<repo>[^/]+)/pull/(?P<number>\d+)/?$`)
)

// Parse auto-detects a GitHub PR or GitLab MR URL.
func Parse(raw string) (Info, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return Info{}, fmt.Errorf("invalid URL (no hostname): %s", raw)
	}
	if u.User != nil {
		return Info{}, fmt.Errorf("MR/PR URLs must not include credentials")
	}
	if u.Hostname() == "github.com" {
		return parseGitHub(u, raw)
	}
	if strings.Contains(u.Path, "/-/merge_requests/") {
		return parseGitLab(u, raw)
	}
	return Info{}, fmt.Errorf("unsupported URL: %s\nExpected a GitHub PR URL (https://github.com/owner/repo/pull/N) or a GitLab MR URL (https://host/group/project/-/merge_requests/N)", raw)
}

// ParseGitLab parses a GitLab MR URL (gitlab.com or self-hosted, nested namespaces).
func ParseGitLab(raw string) (Info, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return Info{}, fmt.Errorf("invalid URL (no hostname): %s", raw)
	}
	if u.User != nil {
		return Info{}, fmt.Errorf("GitLab MR URLs must not include credentials")
	}
	if !strings.Contains(u.Path, "/-/merge_requests/") {
		return Info{}, fmt.Errorf("not a GitLab MR URL (missing /-/merge_requests/ path): %s", raw)
	}
	return parseGitLab(u, raw)
}

// ParseGitHub parses a github.com pull request URL.
func ParseGitHub(raw string) (Info, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return Info{}, fmt.Errorf("invalid URL (no hostname): %s", raw)
	}
	if u.User != nil {
		return Info{}, fmt.Errorf("GitHub PR URLs must not include credentials")
	}
	return parseGitHub(u, raw)
}

func parseGitLab(u *url.URL, raw string) (Info, error) {
	m := gitlabMRPattern.FindStringSubmatch(u.Path)
	if m == nil {
		return Info{}, fmt.Errorf("could not parse GitLab MR URL: %s", raw)
	}
	iid, err := strconv.Atoi(m[gitlabMRPattern.SubexpIndex("iid")])
	if err != nil {
		return Info{}, fmt.Errorf("could not parse GitLab MR URL: %s", raw)
	}
	scheme := u.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return Info{
		Platform:  "gitlab",
		Host:      u.Hostname(),
		BaseURL:   scheme + "://" + u.Host,
		Namespace: m[gitlabMRPattern.SubexpIndex("namespace")],
		Project:   m[gitlabMRPattern.SubexpIndex("project")],
		IID:       iid,
	}, nil
}

func parseGitHub(u *url.URL, raw string) (Info, error) {
	if u.Hostname() != "github.com" {
		return Info{}, fmt.Errorf("not a GitHub PR URL (hostname is not github.com): %s", raw)
	}
	m := githubPRPattern.FindStringSubmatch(u.Path)
	if m == nil {
		return Info{}, fmt.Errorf("could not parse GitHub PR URL: %s", raw)
	}
	n, err := strconv.Atoi(m[githubPRPattern.SubexpIndex("number")])
	if err != nil {
		return Info{}, fmt.Errorf("could not parse GitHub PR URL: %s", raw)
	}
	return Info{
		Platform:  "github",
		Host:      u.Hostname(),
		BaseURL:   "https://github.com",
		Namespace: m[githubPRPattern.SubexpIndex("owner")],
		Project:   m[githubPRPattern.SubexpIndex("repo")],
		IID:       n,
	}, nil
}
