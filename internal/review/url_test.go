package review

import (
	"strings"
	"testing"
)

func TestParseGitLabComSimpleNamespace(t *testing.T) {
	got, err := ParseGitLab("https://gitlab.com/mygroup/myproject/-/merge_requests/42")
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != "gitlab" || got.Host != "gitlab.com" || got.Namespace != "mygroup" || got.Project != "myproject" || got.IID != 42 {
		t.Fatalf("%+v", got)
	}
}

func TestParseGitLabNestedNamespace(t *testing.T) {
	got, err := ParseGitLab("https://gitlab.com/group/subgroup/myproject/-/merge_requests/7")
	if err != nil {
		t.Fatal(err)
	}
	if got.Namespace != "group/subgroup" || got.Project != "myproject" || got.IID != 7 {
		t.Fatalf("%+v", got)
	}
}

func TestParseSelfHostedGitLab(t *testing.T) {
	got, err := ParseGitLab("https://git.example.com/team/repo/-/merge_requests/100")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "git.example.com" || got.Namespace != "team" || got.Project != "repo" || got.IID != 100 {
		t.Fatalf("%+v", got)
	}
}

func TestParseSelfHostedGitLabPreservesSchemeAndPort(t *testing.T) {
	got, err := ParseGitLab("http://git.example.com:8443/team/repo/-/merge_requests/100")
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "git.example.com" || got.BaseURL != "http://git.example.com:8443" {
		t.Fatalf("%+v", got)
	}
}

func TestParseGitLabRejectsEmbeddedCredentials(t *testing.T) {
	_, err := ParseGitLab("https://token@git.example.com/team/repo/-/merge_requests/100")
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGitLabInvalidNoHostname(t *testing.T) {
	_, err := ParseGitLab("not-a-url")
	if err == nil || !strings.Contains(err.Error(), "no hostname") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGitLabNotMergeRequestPath(t *testing.T) {
	_, err := ParseGitLab("https://gitlab.com/group/project/-/issues/1")
	if err == nil || !strings.Contains(err.Error(), "Not a GitLab MR URL") && !strings.Contains(err.Error(), "not a GitLab MR URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGitLabTrailingSlash(t *testing.T) {
	got, err := ParseGitLab("https://gitlab.com/mygroup/myproject/-/merge_requests/5/")
	if err != nil {
		t.Fatal(err)
	}
	if got.IID != 5 || got.Project != "myproject" {
		t.Fatalf("%+v", got)
	}
}

func TestParseGitHubValid(t *testing.T) {
	got, err := ParseGitHub("https://github.com/owner/repo/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != "github" || got.Host != "github.com" || got.Namespace != "owner" || got.Project != "repo" || got.IID != 42 {
		t.Fatalf("%+v", got)
	}
}

func TestParseGitHubTrailingSlash(t *testing.T) {
	got, err := ParseGitHub("https://github.com/owner/repo/pull/99/")
	if err != nil {
		t.Fatal(err)
	}
	if got.IID != 99 {
		t.Fatalf("%+v", got)
	}
}

func TestParseGitHubInvalidHostname(t *testing.T) {
	_, err := ParseGitHub("https://gitlab.com/owner/repo/pull/1")
	if err == nil || !strings.Contains(err.Error(), "not github.com") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGitHubInvalidPath(t *testing.T) {
	_, err := ParseGitHub("https://github.com/owner/repo/issues/1")
	if err == nil || !strings.Contains(err.Error(), "could not parse") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGitHubNoHostname(t *testing.T) {
	_, err := ParseGitHub("not-a-url")
	if err == nil || !strings.Contains(err.Error(), "no hostname") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseGitHubRejectsEmbeddedCredentials(t *testing.T) {
	_, err := Parse("https://token@github.com/owner/repo/pull/1")
	if err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseDetectsGitHub(t *testing.T) {
	got, err := Parse("https://github.com/owner/repo/pull/42")
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != "github" || got.Namespace != "owner" || got.Project != "repo" || got.IID != 42 {
		t.Fatalf("%+v", got)
	}
}

func TestParseDetectsGitLab(t *testing.T) {
	got, err := Parse("https://gitlab.com/group/project/-/merge_requests/1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Platform != "gitlab" || got.Namespace != "group" || got.Project != "project" || got.IID != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestParseGitLabMalformedPathAfterMarker(t *testing.T) {
	_, err := ParseGitLab("https://gitlab.com/-/merge_requests/1")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseUnsupported(t *testing.T) {
	_, err := Parse("https://bitbucket.org/owner/repo/pull-requests/1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
		t.Fatalf("err = %v", err)
	}
}
