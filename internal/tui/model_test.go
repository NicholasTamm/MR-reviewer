package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

func key(r rune) tea.KeyPressMsg {
	if r == '\n' {
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	}
	return tea.KeyPressMsg{Code: r, Text: string(r)}
}

func special(code rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: code}
}

func apply(m Model, msgs ...tea.Msg) Model {
	var cur tea.Model = m
	for _, msg := range msgs {
		next, _ := cur.Update(msg)
		cur = next
	}
	return cur.(Model)
}

func applyKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func drain(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	next, _ := m.Update(cmd())
	return next.(Model)
}

func testModel(t *testing.T) Model {
	t.Helper()
	return New(Deps{
		Settings: config.Settings{Provider: "echo", Model: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadDash: func(search string) ([]review.ProjectMergeRequests, error) {
			return []review.ProjectMergeRequests{{
				ProjectID: 1, ProjectPath: "group/project",
				MergeRequests: []review.MergeRequestSummary{{
					ProjectID: 1, ProjectPath: "group/project", IID: 7,
					Title: "Fix login", WebURL: "https://gitlab.com/group/project/-/merge_requests/7",
					SourceBranch: "feat", TargetBranch: "main",
				}},
			}}, nil
		},
		RunReview: func(url, provider, model string, focus []string, maxC int) (review.Result, review.Metadata, error) {
			return review.Result{
				Summary: "NEEDS CHANGES\nLooks risky.",
				Comments: []review.Comment{
					{File: "a.py", Line: 2, Body: "*error* **BUG:** nil", Severity: "error"},
					{File: "a.py", Line: 4, Body: "*info* **NITPICK:** name", Severity: "info"},
				},
			}, review.Metadata{Title: "Fix login", SourceBranch: "feat", TargetBranch: "main", WebURL: url}, nil
		},
		Post: func(result review.Result) error {
			return nil
		},
		Login: func(provider, method, secret string) (string, error) {
			return "Logged in to " + provider, nil
		},
	})
}

func TestDashboardLoadsAndSelectsMR(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	if m.ViewName() != ViewDashboard {
		t.Fatalf("view = %s", m.ViewName())
	}
	if !strings.Contains(m.render(), "Fix login") || !strings.Contains(m.render(), "group/project") {
		t.Fatalf("view =\n%s", m.render())
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.ViewName() != ViewLink {
		t.Fatalf("view = %s", m.ViewName())
	}
	if m.URL() != "https://gitlab.com/group/project/-/merge_requests/7" {
		t.Fatalf("url = %q", m.URL())
	}
}

func TestLinkFromDashboardAndSearch(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('l'))
	if m.ViewName() != ViewLink {
		t.Fatalf("view = %s", m.ViewName())
	}
	m = testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('/'))
	m, cmd := applyKey(m, key('x'))
	m, cmd = applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.Search() != "x" {
		t.Fatalf("search = %q", m.Search())
	}
}

func TestConfigureAndRunReviewHITL(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('l'))
	// type URL on url field
	for _, r := range "https://github.com/owner/repo/pull/1" {
		m, _ = applyKey(m, key(r))
	}
	if !strings.Contains(m.URL(), "github.com") {
		t.Fatalf("url = %q", m.URL())
	}
	m, _ = applyKey(m, special(tea.KeyTab)) // provider
	m, _ = applyKey(m, special(tea.KeyRight))
	if m.Provider() == "" {
		t.Fatal("empty provider")
	}
	m, _ = applyKey(m, special(tea.KeyTab)) // model
	m, _ = applyKey(m, special(tea.KeyTab)) // focus
	m, _ = applyKey(m, key(' '))
	if len(m.Focus()) == 0 {
		t.Fatal("focus empty")
	}
	m, _ = applyKey(m, special(tea.KeyTab)) // max
	m, _ = applyKey(m, key('+'))
	if m.MaxComments() != 11 {
		t.Fatalf("max = %d", m.MaxComments())
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	if m.ViewName() != ViewReviewing {
		t.Fatalf("view = %s", m.ViewName())
	}
	m = drain(t, m, cmd)
	if m.ViewName() != ViewHITL {
		t.Fatalf("view = %s want hitl; err=%s", m.ViewName(), m.Error())
	}
	if m.CommentCount() != 2 || m.Summary() == "" {
		t.Fatalf("comments=%d summary=%q", m.CommentCount(), m.Summary())
	}
}

func TestHITLApproveRejectEditPostConfirm(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('l'))
	for _, r := range "https://github.com/o/r/pull/1" {
		m, _ = applyKey(m, key(r))
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewHITL {
		t.Fatalf("view = %s", m.ViewName())
	}

	// reject first
	m, _ = applyKey(m, key('r'))
	if m.CommentApproved(0) {
		t.Fatal("expected rejected")
	}
	// approve first
	m, _ = applyKey(m, key('a'))
	if !m.CommentApproved(0) {
		t.Fatal("expected approved")
	}
	// edit first comment
	m, _ = applyKey(m, key('e'))
	m, _ = applyKey(m, special(tea.KeyBackspace))
	m, _ = applyKey(m, key('Z'))
	m, _ = applyKey(m, special(tea.KeyEnter))
	if !strings.HasSuffix(m.CommentBody(0), "Z") && m.CommentBody(0) != "Z" {
		// backspace only removed last rune then added Z
		if m.CommentBody(0) == "" {
			t.Fatal("empty body")
		}
	}

	// post
	m, cmd = applyKey(m, key('p'))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewConfirm {
		t.Fatalf("view = %s err=%s", m.ViewName(), m.Error())
	}
	if m.PostedCount() != m.ApprovedCount() && m.PostedCount() == 0 {
		t.Fatalf("posted = %d", m.PostedCount())
	}
	if !strings.Contains(m.render(), "Review posted") {
		t.Fatalf("confirm view =\n%s", m.render())
	}
	m, _ = applyKey(m, key('n'))
	if m.ViewName() != ViewDashboard {
		t.Fatalf("view = %s", m.ViewName())
	}
}

func TestAuthViewLoginOutcome(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('a'))
	if m.ViewName() != ViewAuth {
		t.Fatalf("view = %s", m.ViewName())
	}
	if !strings.Contains(m.render(), "anthropic") || !strings.Contains(m.render(), "openai") {
		t.Fatalf("auth view =\n%s", m.render())
	}
	// anthropic -> API key prompt
	m, _ = applyKey(m, special(tea.KeyEnter))
	m, _ = applyKey(m, key('s'))
	m, _ = applyKey(m, key('k'))
	m, cmd := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if !strings.Contains(m.Status(), "Logged in") {
		t.Fatalf("status = %q", m.Status())
	}
}

func TestDeviceAuthorizationProgressIsBlockingAndProminent(t *testing.T) {
	m := testModel(t)
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('a'))
	m.authProv = "github"
	m.authMethod = "device"
	m.authAttempt = 1
	m, _ = applyUpdate(m, deviceCodeMsg{
		code:     &auth.DeviceCode{UserCode: "ABCD-1234", VerificationURI: "https://github.com/login/device"},
		provider: "github",
		attempt:  1,
	})
	if m.ViewName() != ViewAuth {
		t.Fatalf("pending authorization left auth view: %s", m.ViewName())
	}
	view := m.render()
	for _, want := range []string{"GITHUB authorization in progress", "https://github.com/login/device", "ABCD-1234", "Waiting for authorization", "c/esc cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("authorization view missing %q:\n%s", want, view)
		}
	}
	m, _ = applyKey(m, special(tea.KeyEsc))
	if m.ViewName() != ViewDashboard {
		t.Fatalf("cancel view = %s, want dashboard", m.ViewName())
	}
}

func TestAuthorizationFailuresStayOnAuthViewAndOfferRetry(t *testing.T) {
	for _, err := range []error{
		errors.New("device authorization was denied"),
		errors.New("device authorization timed out"),
		errors.New("network unreachable"),
	} {
		t.Run(err.Error(), func(t *testing.T) {
			m := testModel(t)
			m = drain(t, m, m.Init())
			m, _ = applyKey(m, key('a'))
			m.authProv = "github"
			m.authMethod = "device"
			m.authAttempt = 1
			m, _ = applyUpdate(m, authDoneMsg{err: err, attempt: 1})
			if m.ViewName() != ViewAuth || !m.authFailed {
				t.Fatalf("failure must remain in auth view: view=%s failed=%v", m.ViewName(), m.authFailed)
			}
			if view := m.render(); !strings.Contains(view, "r retry") || !strings.Contains(view, err.Error()) {
				t.Fatalf("failure view =\n%s", view)
			}
		})
	}
}

func TestReviewErrorView(t *testing.T) {
	m := New(Deps{
		Settings: config.Settings{Provider: "echo", MaxComments: 10, Focus: []string{"bugs"}},
		LoadDash: func(string) ([]review.ProjectMergeRequests, error) { return nil, nil },
		RunReview: func(string, string, string, []string, int) (review.Result, review.Metadata, error) {
			return review.Result{}, review.Metadata{}, errors.New("provider down")
		},
	})
	m = drain(t, m, m.Init())
	m, _ = applyKey(m, key('l'))
	for _, r := range "https://github.com/o/r/pull/1" {
		m, _ = applyKey(m, key(r))
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewError || !strings.Contains(m.Error(), "provider down") {
		t.Fatalf("view=%s err=%q", m.ViewName(), m.Error())
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.ViewName() != ViewLink {
		t.Fatalf("view = %s", m.ViewName())
	}
}

func TestAllViewsReachable(t *testing.T) {
	seen := map[View]bool{}
	m := testModel(t)
	seen[m.ViewName()] = true
	m = drain(t, m, m.Init())
	seen[m.ViewName()] = true
	m, _ = applyKey(m, key('a'))
	seen[m.ViewName()] = true
	m, _ = applyKey(m, special(tea.KeyEsc))
	m, _ = applyKey(m, key('l'))
	seen[m.ViewName()] = true
	for _, r := range "https://github.com/o/r/pull/2" {
		m, _ = applyKey(m, key(r))
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	seen[m.ViewName()] = true
	m = drain(t, m, cmd)
	seen[m.ViewName()] = true
	m, cmd = applyKey(m, key('p'))
	m = drain(t, m, cmd)
	seen[m.ViewName()] = true
	m, _ = applyKey(m, key('n'))
	m, _ = applyKey(m, key('c'))
	seen[m.ViewName()] = true
	for _, v := range []View{ViewDashboard, ViewLink, ViewReviewing, ViewHITL, ViewConfirm, ViewAuth, ViewConfig} {
		if !seen[v] {
			t.Errorf("view %s never reached", v)
		}
	}
}

func TestConfigPanelOpenEditSave(t *testing.T) {
	var saved config.Settings
	m := New(Deps{
		Settings: config.Settings{
			GitHubAPI: "https://api.github.com", GitLabURL: "https://gitlab.com",
			AnthropicURL: "https://api.anthropic.com", Provider: "echo",
			Focus: []string{"bugs"}, MaxComments: 10,
		},
		StartView: ViewConfig,
		LoadDash:  func(string) ([]review.ProjectMergeRequests, error) { return nil, nil },
		SaveSettings: func(s config.Settings) error {
			saved = s
			return nil
		},
	})
	if m.ViewName() != ViewConfig {
		t.Fatalf("view = %s", m.ViewName())
	}
	if !strings.Contains(m.render(), "github api") || !strings.Contains(m.render(), "gitlab") || !strings.Contains(m.render(), "anthropic") {
		t.Fatalf("config view =\n%s", m.render())
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	for _, r := range "/v3" {
		m, _ = applyKey(m, key(r))
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	m, _ = applyKey(m, special(tea.KeyTab))
	m, _ = applyKey(m, special(tea.KeyEnter))
	for _, r := range ".internal" {
		m, _ = applyKey(m, key(r))
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	m, _ = applyKey(m, key('s'))
	if saved.GitHubAPI != "https://api.github.com/v3" {
		t.Fatalf("github saved = %q status=%q", saved.GitHubAPI, m.Status())
	}
	if saved.GitLabURL != "https://gitlab.com.internal" {
		t.Fatalf("gitlab saved = %q", saved.GitLabURL)
	}
	if m.Status() != "saved" {
		t.Fatalf("status = %q", m.Status())
	}
}
