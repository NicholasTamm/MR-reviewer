package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/provider"
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
		LoadProjects: func(platform string) ([]review.Project, error) {
			return []review.Project{{Platform: platform, ID: "1", Path: "group/project"}}, nil
		},
		LoadReviews: func(platform string, project review.Project) ([]review.ReviewSummary, error) {
			return []review.ReviewSummary{{Project: project, Number: 7, Title: "Fix login", WebURL: "https://gitlab.com/group/project/-/merge_requests/7", SourceBranch: "feat", TargetBranch: "main"}}, nil
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

func TestBrowseSelectsProjectAndMR(t *testing.T) {
	m := testModel(t)
	if m.ViewName() != ViewDashboard {
		t.Fatalf("view = %s", m.ViewName())
	}
	if !strings.Contains(m.render(), "GITLAB") {
		t.Fatalf("view =\n%s", m.render())
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewProjects || !strings.Contains(m.render(), "group/project") {
		t.Fatalf("view = %s", m.ViewName())
	}
	m, cmd = applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewReviews || !strings.Contains(m.render(), "Fix login") {
		t.Fatalf("view = %s", m.ViewName())
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.ViewName() != ViewLink || m.URL() != "https://gitlab.com/group/project/-/merge_requests/7" {
		t.Fatalf("view=%s url=%q", m.ViewName(), m.URL())
	}
	if !strings.Contains(m.render(), "Fix login") || strings.Contains(m.render(), "url        https://") {
		t.Fatalf("locked configure view =\n%s", m.render())
	}
}

func TestBrowseConfigureLocksTargetAndIgnoresLeftoverEnter(t *testing.T) {
	var started int
	m := testModel(t)
	m.runReview = func(url, provider, model string, focus []string, maxC int) (review.Result, review.Metadata, error) {
		started++
		return review.Result{Summary: "ok"}, review.Metadata{Title: "Fix login", WebURL: url}, nil
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	m, cmd = applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	m, _ = applyKey(m, special(tea.KeyEnter))
	before := m.URL()
	m, cmd = applyKey(m, special(tea.KeyEnter))
	if started != 0 || m.ViewName() != ViewLink {
		t.Fatalf("leftover enter started review: started=%d view=%s", started, m.ViewName())
	}
	m, _ = applyKey(m, key('x'))
	if m.URL() != before {
		t.Fatalf("locked url changed: %q", m.URL())
	}
	m, _ = applyKey(m, special(tea.KeyTab))
	if m.field == fieldURL {
		t.Fatalf("tab landed on locked url field")
	}
	m, cmd = applyKey(m, special(tea.KeyEnter))
	if cmd == nil || started != 0 {
		t.Fatalf("expected review command after idle enter, started=%d cmd=%v", started, cmd != nil)
	}
	m = drain(t, m, cmd)
	if started != 1 || m.ViewName() != ViewHITL {
		t.Fatalf("review after idle enter: started=%d view=%s", started, m.ViewName())
	}
}

func TestLinkArrowsMoveFieldsAndCycleModels(t *testing.T) {
	m := testModel(t)
	m, _ = applyKey(m, key('l'))
	if m.field != fieldURL {
		t.Fatalf("field=%d", m.field)
	}
	m, _ = applyKey(m, special(tea.KeyDown))
	if m.field != fieldProvider {
		t.Fatalf("down field=%d", m.field)
	}
	m, _ = applyKey(m, key('j'))
	if m.field != fieldProvider {
		t.Fatalf("j must not move fields: %d", m.field)
	}
	before := m.provider
	m, _ = applyKey(m, special(tea.KeyRight))
	if m.provider == before {
		t.Fatalf("provider did not cycle")
	}
	m.field = fieldModel
	m, _ = applyKey(m, special(tea.KeyRight))
	if len(m.models) != 0 || m.modelsAvailable {
		t.Fatalf("unavailable provider received a model catalog: %+v", m.models)
	}
}

func TestUnavailableCatalogDoesNotRestoreBuiltinModels(t *testing.T) {
	m := testModel(t)
	m.provider = "openai"
	m.model = "gpt-4o"
	m.models = nil
	m.modelsAvailable = false
	m.loadModels = func(string) provider.Models {
		return provider.Unavailable("openai", "Unable to retrieve available models.")
	}
	m, cmd := applyKey(m, key('l'))
	m = drain(t, m, cmd)
	if m.modelsAvailable || len(m.models) != 0 || !strings.Contains(m.render(), "Unable to retrieve available models.") {
		t.Fatalf("unavailable catalog = models=%v available=%v view=\n%s", m.models, m.modelsAvailable, m.render())
	}
	m.field = fieldModel
	m, _ = applyKey(m, special(tea.KeyRight))
	if len(m.models) != 0 || m.ModelID() != "gpt-4o" {
		t.Fatalf("cycle restored models=%v selected=%q", m.models, m.ModelID())
	}
}

func TestUnconfiguredProviderIsDimmedAndJumpsToAuth(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var started int
	m := testModel(t)
	m.store = store
	m.provider = "openai"
	m.runReview = func(string, string, string, []string, int) (review.Result, review.Metadata, error) {
		started++
		return review.Result{}, review.Metadata{}, nil
	}
	m, _ = applyKey(m, key('l'))
	if strings.Contains(m.render(), "unconfigured") == false || !strings.Contains(m.render(), "model      -") {
		t.Fatalf("unconfigured view =\n%s", m.render())
	}
	if m.loadModels != nil {
		t.Fatal("test fixture should not fetch")
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	if started != 0 || cmd != nil || m.ViewName() != ViewAuth || m.authProv != "openai" {
		t.Fatalf("enter should open auth: started=%d view=%s prov=%s", started, m.ViewName(), m.authProv)
	}
	m, _ = applyKey(m, special(tea.KeyEsc))
	if m.ViewName() != ViewLink {
		t.Fatalf("esc back to configure = %s", m.ViewName())
	}
}

func TestManualLinkURLRemainsEditable(t *testing.T) {
	m := testModel(t)
	m, _ = applyKey(m, key('l'))
	m, _ = applyKey(m, key('h'))
	if m.URL() != "h" {
		t.Fatalf("manual url = %q", m.URL())
	}
}

func TestPlatformSwitchIsScopedToBrowseEntry(t *testing.T) {
	m := testModel(t)
	m, _ = applyKey(m, special(tea.KeyTab))
	if !strings.Contains(m.render(), "GITHUB") {
		t.Fatalf("view =\n%s", m.render())
	}
	m, _ = applyKey(m, key('l'))
	m, _ = applyKey(m, special(tea.KeyTab))
	if m.field != fieldProvider {
		t.Fatalf("link tab changed field=%d", m.field)
	}
}

func TestBrowseIgnoresStaleCatalogResponses(t *testing.T) {
	m := testModel(t)
	m, firstProjects := applyKey(m, special(tea.KeyEnter))
	m, _ = applyKey(m, special(tea.KeyEsc))
	m, _ = applyKey(m, special(tea.KeyTab))
	m, secondProjects := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, secondProjects)
	m = drain(t, m, firstProjects)
	if m.ViewName() != ViewProjects || m.projects[0].Platform != "github" {
		t.Fatalf("stale project response replaced active selection: platform=%q request=%d view=%s", m.projects[0].Platform, m.catalogRequest, m.ViewName())
	}

	m, firstReviews := applyKey(m, special(tea.KeyEnter))
	m, secondReviews := applyKey(m, key('r'))
	m = drain(t, m, secondReviews)
	m = drain(t, m, firstReviews)
	if m.ViewName() != ViewReviews || m.catalogRequest != 4 {
		t.Fatalf("stale review response replaced active request: view=%s request=%d", m.ViewName(), m.catalogRequest)
	}
}

func TestBrowseBackNavigationAndCatalogStates(t *testing.T) {
	m := testModel(t)
	m, cmd := applyKey(m, special(tea.KeyEnter))
	if m.ViewName() != ViewProjects || !strings.Contains(m.render(), "loading projects") {
		t.Fatalf("project loading view =\n%s", m.render())
	}
	m = drain(t, m, cmd)
	m, cmd = applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewReviews || !strings.Contains(m.render(), "Fix login") {
		t.Fatalf("review view =\n%s", m.render())
	}
	m, _ = applyKey(m, special(tea.KeyEsc))
	if m.ViewName() != ViewProjects {
		t.Fatalf("back from reviews = %s", m.ViewName())
	}
	m, _ = applyKey(m, special(tea.KeyEsc))
	if m.ViewName() != ViewDashboard {
		t.Fatalf("back from projects = %s", m.ViewName())
	}

	empty := New(Deps{
		Settings:     config.Settings{Provider: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadProjects: func(string) ([]review.Project, error) { return nil, nil },
	})
	empty, cmd = applyKey(empty, special(tea.KeyEnter))
	empty = drain(t, empty, cmd)
	if !strings.Contains(empty.render(), "no accessible projects") {
		t.Fatalf("empty view =\n%s", empty.render())
	}

	failed := New(Deps{
		Settings:     config.Settings{Provider: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadProjects: func(string) ([]review.Project, error) { return nil, errors.New("network unavailable") },
	})
	failed, cmd = applyKey(failed, special(tea.KeyEnter))
	failed = drain(t, failed, cmd)
	if !strings.Contains(failed.render(), "network unavailable") {
		t.Fatalf("failed view =\n%s", failed.render())
	}
	failed, cmd = applyKey(failed, key('r'))
	if cmd == nil || failed.Status() != "" {
		t.Fatalf("retry command=%v status=%q", cmd != nil, failed.Status())
	}

	reviewState := New(Deps{
		Settings: config.Settings{Provider: "echo", Focus: []string{"bugs"}, MaxComments: 10},
		LoadProjects: func(string) ([]review.Project, error) {
			return []review.Project{{Platform: "gitlab", ID: "1", Path: "group/project"}}, nil
		},
		LoadReviews: func(string, review.Project) ([]review.ReviewSummary, error) {
			return nil, errors.New("authorization failed")
		},
	})
	reviewState, cmd = applyKey(reviewState, special(tea.KeyEnter))
	reviewState = drain(t, reviewState, cmd)
	reviewState, cmd = applyKey(reviewState, special(tea.KeyEnter))
	if !strings.Contains(reviewState.render(), "loading merge requests") {
		t.Fatalf("review loading view =\n%s", reviewState.render())
	}
	reviewState = drain(t, reviewState, cmd)
	if !strings.Contains(reviewState.render(), "authorization failed") {
		t.Fatalf("review error view =\n%s", reviewState.render())
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

func TestOnboardingRoutesIncompleteStateAndCompletesWithKeyboard(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved config.Settings
	m := New(Deps{
		Store: store,
		Settings: config.Settings{
			Provider: "anthropic", GitHubAPI: "https://api.github.com", GitLabURL: "https://gitlab.com",
			Focus: []string{"bugs"}, MaxComments: 10,
		},
		SaveSettings:    func(next config.Settings) error { saved = next; return nil },
		CheckOnboarding: true,
	})
	if m.ViewName() != ViewOnboarding {
		t.Fatalf("incomplete shared state view = %s", m.ViewName())
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.input != inputOnboardingSecret || !strings.Contains(m.Status(), "anthropic") {
		t.Fatalf("provider prompt input=%d status=%q", m.input, m.Status())
	}
	for _, r := range "provider-secret" {
		m, _ = applyKey(m, key(r))
	}
	if view := m.render(); strings.Contains(view, "provider-secret") {
		t.Fatalf("provider secret exposed in view:\n%s", view)
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.onboardingStep != onboardingPlatform || m.input != inputNone {
		t.Fatalf("after provider step=%d input=%d", m.onboardingStep, m.input)
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	if m.input != inputOnboardingSecret || !strings.Contains(m.Status(), "github") {
		t.Fatalf("platform prompt input=%d status=%q", m.input, m.Status())
	}
	m, _ = applyKey(m, special(tea.KeyEsc))
	if m.onboardingStep != onboardingPlatform || !strings.Contains(m.render(), "Select a Git platform") {
		t.Fatalf("cancel platform credential step=%d view=\n%s", m.onboardingStep, m.render())
	}
	m, _ = applyKey(m, special(tea.KeyEnter))
	for _, r := range "platform-secret" {
		m, _ = applyKey(m, key(r))
	}
	if view := m.render(); strings.Contains(view, "platform-secret") {
		t.Fatalf("platform secret exposed in view:\n%s", view)
	}
	m, cmd := applyKey(m, special(tea.KeyEnter))
	m = drain(t, m, cmd)
	if m.ViewName() != ViewDashboard {
		t.Fatalf("completed onboarding view = %s status=%q", m.ViewName(), m.Status())
	}
	if !saved.OnboardingStatus(store).Complete {
		t.Fatalf("saved onboarding status = %+v", saved.OnboardingStatus(store))
	}
}

func TestOnboardingBypassesCompleteSharedState(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Set("anthropic", auth.Credential{Type: auth.TypeAPIKey, APIKey: "provider"}); err != nil {
		t.Fatal(err)
	}
	target, err := auth.PublicTarget("github")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: "platform"}); err != nil {
		t.Fatal(err)
	}
	providerFingerprint, _ := auth.CredentialFingerprint("anthropic", store, "")
	platformFingerprint, _ := auth.PlatformCredentialFingerprint(target, store)
	cfg := config.Settings{
		Provider: "anthropic", GitHubAPI: "https://api.github.com", GitLabURL: "https://gitlab.com",
		Focus: []string{"bugs"}, MaxComments: 10,
		Onboarding: config.OnboardingState{
			SchemaVersion: config.OnboardingSchemaVersion, Provider: "anthropic", ProviderFingerprint: providerFingerprint,
			ProviderValidatedAt: time.Now(), Platform: "github", PlatformOrigin: target.Origin, PlatformAPIBase: target.APIBase,
			PlatformFingerprint: platformFingerprint, PlatformValidatedAt: time.Now(),
		},
	}
	m := New(Deps{Store: store, Settings: cfg, CheckOnboarding: true})
	if m.ViewName() != ViewDashboard {
		t.Fatalf("complete shared state view = %s", m.ViewName())
	}
	cfg.Onboarding.ProviderFingerprint = "stale"
	m = New(Deps{Store: store, Settings: cfg, CheckOnboarding: true})
	if m.ViewName() != ViewOnboarding {
		t.Fatalf("stale shared credentials view = %s", m.ViewName())
	}
}

func TestConfigCannotBypassIncompleteOnboarding(t *testing.T) {
	store, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := New(Deps{
		Store: store,
		Settings: config.Settings{
			Provider: "anthropic", GitHubAPI: "https://api.github.com", GitLabURL: "https://gitlab.com",
		},
		StartView:       ViewConfig,
		CheckOnboarding: true,
	})
	m, _ = applyKey(m, special(tea.KeyEsc))
	if m.ViewName() != ViewOnboarding {
		t.Fatalf("config escape view = %s", m.ViewName())
	}
}
