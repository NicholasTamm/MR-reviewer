package tui

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/provider"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

// View is a reachable TUI screen.
type View int

const (
	ViewDashboard View = iota
	ViewLink
	ViewReviewing
	ViewHITL
	ViewConfirm
	ViewAuth
	ViewError
	ViewConfig
	ViewOnboarding
	ViewProjects
	ViewReviews
)

func (v View) String() string {
	switch v {
	case ViewDashboard:
		return "dashboard"
	case ViewLink:
		return "link"
	case ViewReviewing:
		return "reviewing"
	case ViewHITL:
		return "hitl"
	case ViewConfirm:
		return "confirm"
	case ViewAuth:
		return "auth"
	case ViewError:
		return "error"
	case ViewConfig:
		return "config"
	case ViewOnboarding:
		return "onboarding"
	case ViewProjects:
		return "projects"
	case ViewReviews:
		return "reviews"
	default:
		return "unknown"
	}
}

type field int

const (
	fieldURL field = iota
	fieldProvider
	fieldModel
	fieldFocus
	fieldMax
	fieldAutoPost
)

type inputMode int

const (
	inputNone inputMode = iota
	inputURL
	inputModel
	inputEditComment
	inputEditSummary
	inputAPIKey
	inputOAuthPaste
	inputConfig
	inputOnboardingSecret
)

type configField int

const (
	cfgFieldGitHub configField = iota
	cfgFieldGitLab
	cfgFieldAnthropic
)

type onboardingStep int

const (
	onboardingProvider onboardingStep = iota
	onboardingPlatform
	onboardingSecret
)

type hitlComment struct {
	review.Comment
	Approved bool
}

// Model is the Bubble Tea 2 application.
type Model struct {
	width, height int
	view          View
	err           string
	status        string

	cfg   config.Settings
	store *auth.Store

	// browse
	platform       string
	projects       []review.Project
	reviews        []review.ReviewSummary
	project        review.Project
	cursor         int
	catalogLoaded  bool
	catalogRequest uint64

	// configure / link
	url         string
	reviewTitle string
	urlLocked   bool
	ignoreEnter bool
	provider    string
	model       string
	focus       []string
	maxC        int
	autoPost    bool
	field       field

	// HITL
	summary   string
	comments  []hitlComment
	meta      review.Metadata
	postedN   int
	editBuf   string
	input     inputMode
	reviewing bool

	// auth
	authList    []string
	authCursor  int
	pending     *auth.PendingLogin
	authProv    string
	authMethod  string
	authCode    *auth.DeviceCode
	authCtx     context.Context
	authCancel  context.CancelFunc
	authAttempt int
	authFailed  bool

	// injectable for tests / wiring
	loadProjects    func(platform string) ([]review.Project, error)
	loadReviews     func(platform string, project review.Project) ([]review.ReviewSummary, error)
	runReview       func(url, provider, model string, focus []string, maxC int) (review.Result, review.Metadata, error)
	postFn          func(result review.Result) error
	loginFn         func(provider, method, secret string) (string, error)
	beginOAuth      func(provider string) (*auth.PendingLogin, error)
	device          auth.DeviceConfig
	saveFn          func(config.Settings) error
	loadModels      func(provider string) provider.Models
	models          []string
	modelsAvailable bool
	modelRequest    uint64
	configureAuth   bool

	// config panel
	cfgField     configField
	cfgGitHub    string
	cfgGitLab    string
	cfgAnthropic string

	// first-run onboarding
	onboardingStep     onboardingStep
	onboardingCursor   int
	onboardingProvider string
	onboardingPlatform string
}

type projectsMsg struct {
	platform string
	request  uint64
	projects []review.Project
	err      error
}

type reviewsMsg struct {
	platform string
	project  review.Project
	request  uint64
	reviews  []review.ReviewSummary
	err      error
}

type modelsMsg struct {
	provider string
	request  uint64
	catalog  provider.Models
}

type reviewDoneMsg struct {
	result review.Result
	meta   review.Metadata
	err    error
}

type postDoneMsg struct {
	n   int
	err error
}

type authDoneMsg struct {
	message string
	err     error
	attempt int
}

type deviceCodeMsg struct {
	code     *auth.DeviceCode
	cfg      auth.DeviceConfig
	provider string
	attempt  int
}

type Deps struct {
	Store           *auth.Store
	Settings        config.Settings
	LoadProjects    func(platform string) ([]review.Project, error)
	LoadReviews     func(platform string, project review.Project) ([]review.ReviewSummary, error)
	RunReview       func(url, provider, model string, focus []string, maxC int) (review.Result, review.Metadata, error)
	Post            func(result review.Result) error
	Login           func(provider, method, secret string) (string, error)
	BeginOAuth      func(provider string) (*auth.PendingLogin, error)
	Device          auth.DeviceConfig
	StartView       View
	SaveSettings    func(config.Settings) error
	CheckOnboarding bool
	LoadModels      func(provider string) provider.Models
}

func New(deps Deps) Model {
	cfg := deps.Settings
	if cfg.Provider == "" {
		cfg = config.Load()
	}
	m := Model{
		width:           80,
		height:          24,
		view:            ViewDashboard,
		cfg:             cfg,
		store:           deps.Store,
		provider:        cfg.Provider,
		model:           cfg.Model,
		focus:           append([]string{}, cfg.Focus...),
		maxC:            cfg.MaxComments,
		platform:        "gitlab",
		loadProjects:    deps.LoadProjects,
		loadReviews:     deps.LoadReviews,
		runReview:       deps.RunReview,
		postFn:          deps.Post,
		loginFn:         deps.Login,
		beginOAuth:      deps.BeginOAuth,
		device:          deps.Device,
		saveFn:          deps.SaveSettings,
		loadModels:      deps.LoadModels,
		modelsAvailable: config.CanonicalProviderID(cfg.Provider) == "echo",
		cfgGitHub:       cfg.GitHubAPI,
		cfgGitLab:       cfg.GitLabURL,
		cfgAnthropic:    cfg.AnthropicURL,
		authList:        append(auth.BuiltinProviders(), "gitlab", "github"),
	}
	if cfg.Onboarding.Platform == "github" || cfg.Onboarding.Platform == "gitlab" {
		m.platform = cfg.Onboarding.Platform
	}
	if deps.StartView == ViewConfig {
		m.view = ViewConfig
	} else if deps.CheckOnboarding && deps.Store != nil && !cfg.OnboardingStatus(deps.Store).Complete {
		m.view = ViewOnboarding
	}
	if m.maxC <= 0 {
		m.maxC = 10
	}
	if len(m.focus) == 0 {
		m.focus = append([]string{}, review.DefaultFocus...)
	}
	if m.provider == "" {
		m.provider = "anthropic"
	}
	if m.model == "" {
		m.model = provider.DefaultModel(m.provider)
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) ViewName() View { return m.view }

func (m Model) fetchProjects() tea.Cmd {
	if m.loadProjects == nil {
		return nil
	}
	platform, request := m.platform, m.catalogRequest
	return func() tea.Msg {
		projects, err := m.loadProjects(platform)
		return projectsMsg{platform: platform, request: request, projects: projects, err: err}
	}
}

func (m Model) fetchReviews() tea.Cmd {
	if m.loadReviews == nil {
		return nil
	}
	platform, project, request := m.platform, m.project, m.catalogRequest
	return func() tea.Msg {
		reviews, err := m.loadReviews(platform, project)
		return reviewsMsg{platform: platform, project: project, request: request, reviews: reviews, err: err}
	}
}

func (m Model) startReview() tea.Cmd {
	if m.runReview == nil {
		return func() tea.Msg { return reviewDoneMsg{err: context.Canceled} }
	}
	url, prov, model, focus, maxC := m.url, m.provider, m.model, append([]string{}, m.focus...), m.maxC
	return func() tea.Msg {
		res, meta, err := m.runReview(url, prov, model, focus, maxC)
		return reviewDoneMsg{result: res, meta: meta, err: err}
	}
}

func (m Model) startPost() tea.Cmd {
	var approved []review.Comment
	for _, c := range m.comments {
		if c.Approved {
			approved = append(approved, c.Comment)
		}
	}
	res := review.Result{Summary: m.summary, Comments: approved}
	postFn := m.postFn
	return func() tea.Msg {
		if postFn == nil {
			return postDoneMsg{n: len(approved)}
		}
		if err := postFn(res); err != nil {
			return postDoneMsg{err: err}
		}
		return postDoneMsg{n: len(approved)}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case projectsMsg:
		if msg.platform != m.platform || msg.request != m.catalogRequest || m.view != ViewProjects {
			return m, nil
		}
		if msg.err != nil {
			m.status = msg.err.Error()
			m.catalogLoaded = true
			return m, nil
		}
		m.projects = msg.projects
		m.cursor = 0
		m.catalogLoaded = true
		m.status = ""
		return m, nil
	case modelsMsg:
		if msg.provider != m.provider || msg.request != m.modelRequest || m.view != ViewLink {
			return m, nil
		}
		m.models = msg.catalog.Models
		m.modelsAvailable = msg.catalog.Available
		if msg.catalog.Error != "" {
			m.status = msg.catalog.Error
		} else if m.status == "loading models…" {
			m.status = ""
		}
		if m.modelsAvailable && !containsString(m.models, m.model) && len(m.models) > 0 {
			m.model = m.models[0]
		}
		return m, nil
	case reviewsMsg:
		if msg.platform != m.platform || msg.project.ID != m.project.ID || msg.request != m.catalogRequest || m.view != ViewReviews {
			return m, nil
		}
		if msg.err != nil {
			m.status = msg.err.Error()
			m.catalogLoaded = true
			return m, nil
		}
		m.reviews = msg.reviews
		m.cursor = 0
		m.catalogLoaded = true
		m.status = ""
		return m, nil
	case reviewDoneMsg:
		m.reviewing = false
		if msg.err != nil {
			m.err = msg.err.Error()
			m.view = ViewError
			return m, nil
		}
		m.summary = msg.result.Summary
		m.meta = msg.meta
		m.comments = m.comments[:0]
		for _, c := range msg.result.Comments {
			m.comments = append(m.comments, hitlComment{Comment: c, Approved: true})
		}
		m.cursor = 0
		if m.autoPost {
			m.view = ViewReviewing
			return m, m.startPost()
		}
		m.view = ViewHITL
		return m, nil
	case postDoneMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
			m.view = ViewError
			return m, nil
		}
		m.postedN = msg.n
		m.view = ViewConfirm
		return m, nil
	case deviceCodeMsg:
		if msg.attempt != m.authAttempt {
			return m, nil
		}
		m.authCode = msg.code
		m.authFailed = false
		m.status = "Waiting for authorization... (code " + msg.code.UserCode + ")"
		if m.authCtx == nil {
			m.authCtx, m.authCancel = context.WithCancel(context.Background())
		}
		ctx := m.authCtx
		store := m.store
		cfg := msg.cfg
		code := msg.code
		return m, func() tea.Msg {
			tok, err := cfg.Poll(ctx, code)
			if err != nil {
				return authDoneMsg{err: err, attempt: msg.attempt}
			}
			var out string
			if msg.provider == "gitlab" || msg.provider == "github" {
				target, _ := auth.PublicTarget(msg.provider)
				out, err = auth.CompletePlatformLogin(ctx, store, target, msg.cfg.ClientID, tok)
			} else {
				out, err = PersistLogin(ctx, store, "xai", "device", tok, "")
			}
			return authDoneMsg{message: out, err: err, attempt: msg.attempt}
		}
	case authDoneMsg:
		if msg.attempt != 0 && msg.attempt != m.authAttempt {
			return m, nil
		}
		m.authCancel = nil
		m.authCtx = nil
		if msg.err != nil {
			m.authFailed = true
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = msg.message
		m.pending = nil
		m.authCode = nil
		m.authFailed = false
		m.input = inputNone
		if m.configureAuth {
			m.configureAuth = false
			m.view = ViewLink
			return m, m.fetchModels()
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		return m.handlePaste(msg.Content)
	}
	return m, nil
}

func (m Model) handlePaste(s string) (tea.Model, tea.Cmd) {
	s = strings.TrimSpace(s)
	switch m.input {
	case inputURL:
		if !m.urlLocked {
			m.url += s
		}
	case inputModel:
		m.model += s
	case inputEditComment, inputEditSummary:
		m.editBuf += s
	case inputAPIKey:
		m.editBuf += s
	case inputOAuthPaste:
		m.editBuf += s
	case inputConfig:
		m.appendConfigField(s)
	case inputOnboardingSecret:
		m.editBuf += s
	default:
		if m.view == ViewLink && m.field == fieldURL && !m.urlLocked {
			m.url += s
		}
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.Keystroke() == "ctrl+c" {
		return m, tea.Quit
	}

	if m.input != inputNone {
		return m.handleInput(msg)
	}

	switch m.view {
	case ViewDashboard:
		return m.keysDashboard(msg)
	case ViewProjects:
		return m.keysProjects(msg)
	case ViewReviews:
		return m.keysReviews(msg)
	case ViewLink:
		return m.keysLink(msg)
	case ViewReviewing:
		if msg.Code == tea.KeyEsc {
			m.view = ViewLink
			return m, nil
		}
	case ViewHITL:
		return m.keysHITL(msg)
	case ViewConfirm:
		return m.keysConfirm(msg)
	case ViewAuth:
		return m.keysAuth(msg)
	case ViewConfig:
		return m.keysConfig(msg)
	case ViewOnboarding:
		return m.keysOnboarding(msg)
	case ViewError:
		if msg.Code == tea.KeyEnter || msg.Code == tea.KeyEsc {
			m.view = ViewLink
			m.err = ""
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.Code {
	case tea.KeyEsc:
		if m.input == inputOnboardingSecret && m.onboardingStep == onboardingSecret {
			m.onboardingStep = onboardingPlatform
		}
		m.input = inputNone
		m.editBuf = ""
		return m, nil
	case tea.KeyEnter:
		return m.commitInput()
	case tea.KeyBackspace:
		switch m.input {
		case inputURL:
			if !m.urlLocked {
				m.url = trimLast(m.url)
			}
		case inputModel:
			m.model = trimLast(m.model)
		case inputConfig:
			m.trimConfigField()
		default:
			m.editBuf = trimLast(m.editBuf)
		}
		return m, nil
	}
	ch := keyText(msg)
	if ch == "" {
		return m, nil
	}
	switch m.input {
	case inputURL:
		if !m.urlLocked {
			m.url += ch
		}
	case inputModel:
		m.model += ch
	case inputConfig:
		m.appendConfigField(ch)
	case inputOnboardingSecret:
		m.editBuf += ch
	default:
		m.editBuf += ch
	}
	return m, nil
}

func (m Model) commitInput() (tea.Model, tea.Cmd) {
	switch m.input {
	case inputURL, inputModel, inputConfig:
		m.input = inputNone
		return m, nil
	case inputEditComment:
		if m.cursor >= 0 && m.cursor < len(m.comments) {
			m.comments[m.cursor].Body = m.editBuf
		}
		m.input = inputNone
		m.editBuf = ""
		return m, nil
	case inputEditSummary:
		m.summary = m.editBuf
		m.input = inputNone
		m.editBuf = ""
		return m, nil
	case inputAPIKey:
		key := strings.TrimSpace(m.editBuf)
		m.input = inputNone
		m.editBuf = ""
		if m.loginFn == nil || key == "" {
			return m, nil
		}
		prov := m.authProv
		return m, func() tea.Msg {
			msg, err := m.loginFn(prov, "key", key)
			return authDoneMsg{message: msg, err: err}
		}
	case inputOAuthPaste:
		paste := strings.TrimSpace(m.editBuf)
		m.editBuf = ""
		if m.pending == nil {
			m.input = inputNone
			return m, nil
		}
		if err := m.pending.CompleteWithPaste(paste); err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.status = "Exchanging code…"
		m.input = inputNone
		return m, m.waitOAuth(m.authProv, m.pending)
	case inputOnboardingSecret:
		secret := strings.TrimSpace(m.editBuf)
		m.editBuf = ""
		m.input = inputNone
		if secret == "" {
			m.status = "credential is required"
			return m, nil
		}
		if m.store == nil {
			m.status = "credential store is unavailable"
			return m, nil
		}
		if m.onboardingStep == onboardingProvider {
			if err := m.store.Set(m.onboardingProvider, auth.Credential{Type: auth.TypeAPIKey, APIKey: secret}); err != nil {
				m.status = "could not save provider credential; try again"
				return m, nil
			}
			return m.finishProviderOnboarding()
		}
		target, err := m.onboardingPlatformTarget()
		if err != nil {
			m.status = "platform configuration is invalid; repair it in config"
			return m, nil
		}
		if err := m.store.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: secret}); err != nil {
			m.status = "could not save platform credential; try again"
			return m, nil
		}
		return m.finishPlatformOnboarding()
	}
	m.input = inputNone
	return m, nil
}

func (m Model) keysDashboard(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc || keyText(msg) == "q":
		return m, tea.Quit
	case msg.Code == tea.KeyTab:
		if m.platform == "gitlab" {
			m.platform = "github"
		} else {
			m.platform = "gitlab"
		}
		return m, nil
	case msg.Code == tea.KeyEnter:
		m.view = ViewProjects
		m.cursor = 0
		m.catalogLoaded = false
		m.status = ""
		m.catalogRequest++
		return m, m.fetchProjects()
	case keyText(msg) == "l":
		m.view = ViewLink
		m.field = fieldURL
		m.urlLocked = false
		m.reviewTitle = ""
		m.ignoreEnter = false
		return m, m.fetchModels()
	case keyText(msg) == "a":
		m.view = ViewAuth
		m.authCursor = 0
		return m, nil
	case keyText(msg) == "c":
		m.view = ViewConfig
		m.cfgField = cfgFieldGitHub
		m.cfgGitHub = m.cfg.GitHubAPI
		m.cfgGitLab = m.cfg.GitLabURL
		m.cfgAnthropic = m.cfg.AnthropicURL
		return m, nil
	}
	return m, nil
}

func (m Model) keysProjects(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		m.view = ViewDashboard
	case keyText(msg) == "r":
		m.catalogLoaded = false
		m.status = ""
		m.catalogRequest++
		return m, m.fetchProjects()
	case msg.Code == tea.KeyDown || keyText(msg) == "j":
		if m.cursor < len(m.projects)-1 {
			m.cursor++
		}
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case msg.Code == tea.KeyEnter && m.catalogLoaded && m.status == "" && m.cursor < len(m.projects):
		m.project = m.projects[m.cursor]
		m.view = ViewReviews
		m.cursor = 0
		m.catalogLoaded = false
		m.catalogRequest++
		return m, m.fetchReviews()
	}
	return m, nil
}

func (m Model) keysReviews(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		m.view = ViewProjects
		m.cursor = 0
	case keyText(msg) == "r":
		m.catalogLoaded = false
		m.status = ""
		m.catalogRequest++
		return m, m.fetchReviews()
	case msg.Code == tea.KeyDown || keyText(msg) == "j":
		if m.cursor < len(m.reviews)-1 {
			m.cursor++
		}
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case msg.Code == tea.KeyEnter && m.catalogLoaded && m.status == "" && m.cursor < len(m.reviews):
		selected := m.reviews[m.cursor]
		m.url = selected.WebURL
		m.reviewTitle = selected.Title
		m.urlLocked = true
		m.ignoreEnter = true
		m.view = ViewLink
		m.field = fieldProvider
		return m, m.fetchModels()
	}
	return m, nil
}

func (m Model) keysLink(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.ignoreEnter && msg.Code == tea.KeyEnter {
		m.ignoreEnter = false
		return m, nil
	}
	if msg.Code != tea.KeyEnter {
		m.ignoreEnter = false
	}
	if !m.modelsAvailable && !m.providerConfigured() && config.CanonicalProviderID(m.provider) != "ollama" && keyText(msg) == "a" {
		return m.startConfigureAuth()
	}
	switch {
	case msg.Code == tea.KeyEsc:
		if m.urlLocked {
			m.view = ViewReviews
		} else {
			m.view = ViewDashboard
		}
		return m, nil
	case msg.Code == tea.KeyTab:
		if msg.Mod&tea.ModShift != 0 {
			m.retreatLinkField()
		} else {
			m.advanceLinkField()
		}
		return m, nil
	case msg.Code == tea.KeyDown:
		m.advanceLinkField()
		return m, nil
	case msg.Code == tea.KeyUp:
		m.retreatLinkField()
		return m, nil
	case msg.Code == tea.KeyEnter:
		if !m.modelsAvailable {
			if m.providerConfigured() || config.CanonicalProviderID(m.provider) == "ollama" {
				if m.status == "" || m.status == "loading models…" {
					m.status = "Provider models are unavailable."
				}
				return m, nil
			}
			return m.startConfigureAuth()
		}
		if m.field == fieldURL && !m.urlLocked && m.url == "" {
			m.input = inputURL
			return m, nil
		}
		if m.field == fieldModel && strings.TrimSpace(m.model) == "" {
			m.input = inputModel
			return m, nil
		}
		if strings.TrimSpace(m.url) == "" {
			m.status = "paste a GitHub PR or GitLab MR URL"
			return m, nil
		}
		m.view = ViewReviewing
		m.reviewing = true
		return m, m.startReview()
	case keyText(msg) == " ":
		if m.field == fieldFocus {
			m.toggleFocus()
		}
		if m.field == fieldAutoPost {
			m.autoPost = !m.autoPost
		}
		return m, nil
	case keyText(msg) == "+" || keyText(msg) == "=":
		if m.maxC < 50 {
			m.maxC++
		}
		return m, nil
	case keyText(msg) == "-" || keyText(msg) == "_":
		if m.maxC > 1 {
			m.maxC--
		}
		return m, nil
	case msg.Code == tea.KeyLeft:
		return m.cycleLinkValue(-1)
	case msg.Code == tea.KeyRight:
		return m.cycleLinkValue(1)
	}
	if m.field == fieldURL && !m.urlLocked && keyText(msg) != "" && msg.Code != tea.KeySpace {
		m.url += keyText(msg)
	}
	if m.field == fieldModel && m.modelsAvailable && keyText(msg) != "" && msg.Code != tea.KeySpace {
		m.input = inputModel
		m.model += keyText(msg)
	}
	return m, nil
}

func (m *Model) advanceLinkField() {
	for {
		m.field = (m.field + 1) % 6
		if m.field != fieldURL || !m.urlLocked {
			return
		}
	}
}

func (m *Model) retreatLinkField() {
	for {
		m.field = (m.field + 5) % 6
		if m.field != fieldURL || !m.urlLocked {
			return
		}
	}
}

func (m Model) cycleLinkValue(delta int) (tea.Model, tea.Cmd) {
	switch m.field {
	case fieldProvider:
		m.cycleProvider(delta)
		return m, m.fetchModels()
	case fieldModel:
		if m.modelsAvailable {
			m.cycleModel(delta)
		}
	case fieldMax:
		if delta > 0 && m.maxC < 50 {
			m.maxC++
		}
		if delta < 0 && m.maxC > 1 {
			m.maxC--
		}
	case fieldAutoPost:
		m.autoPost = !m.autoPost
	case fieldFocus:
		m.toggleFocus()
	}
	return m, nil
}

func (m *Model) fetchModels() tea.Cmd {
	m.modelRequest++
	request, name := m.modelRequest, m.provider
	m.models = nil
	m.modelsAvailable = false
	if name == "echo" {
		m.models = []string{"echo"}
		m.modelsAvailable = true
		return nil
	}
	if m.loadModels == nil {
		return nil
	}
	m.status = "loading models…"
	return func() tea.Msg {
		return modelsMsg{provider: name, request: request, catalog: m.loadModels(name)}
	}
}

func (m *Model) cycleModel(delta int) {
	if len(m.models) == 0 {
		return
	}
	i := 0
	for j, id := range m.models {
		if id == m.model {
			i = j
			break
		}
	}
	m.model = m.models[(i+delta+len(m.models))%len(m.models)]
}

func (m Model) keysHITL(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		m.view = ViewLink
		return m, nil
	case msg.Code == tea.KeyDown || keyText(msg) == "j":
		if m.cursor < len(m.comments)-1 {
			m.cursor++
		}
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case keyText(msg) == "a":
		if m.cursor >= 0 && m.cursor < len(m.comments) {
			m.comments[m.cursor].Approved = true
		}
	case keyText(msg) == "r":
		if m.cursor >= 0 && m.cursor < len(m.comments) {
			m.comments[m.cursor].Approved = false
		}
	case keyText(msg) == "e":
		if m.cursor >= 0 && m.cursor < len(m.comments) {
			m.input = inputEditComment
			m.editBuf = m.comments[m.cursor].Body
		}
	case keyText(msg) == "s":
		m.input = inputEditSummary
		m.editBuf = m.summary
	case keyText(msg) == "p":
		return m, m.startPost()
	}
	return m, nil
}

func (m Model) keysConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch keyText(msg) {
	case "n", "":
		if msg.Code == tea.KeyEnter || keyText(msg) == "n" {
			m.view = ViewDashboard
			m.url = ""
			m.comments = nil
			return m, nil
		}
	case "q":
		return m, tea.Quit
	}
	if msg.Code == tea.KeyEnter {
		m.view = ViewDashboard
		m.url = ""
		m.comments = nil
		return m, nil
	}
	return m, nil
}

func (m Model) leaveAuthView() Model {
	if m.configureAuth {
		m.configureAuth = false
		m.view = ViewLink
	} else {
		m.view = ViewDashboard
	}
	return m
}

func (m Model) startConfigureAuth() (tea.Model, tea.Cmd) {
	m.configureAuth = true
	m.authProv = m.provider
	for i, name := range m.authList {
		if name == m.provider {
			m.authCursor = i
			break
		}
	}
	m.view = ViewAuth
	m.status = "add credentials for " + m.provider
	return m, nil
}

func (m Model) providerConfigured() bool {
	name := config.CanonicalProviderID(m.provider)
	if name == "echo" {
		return true
	}
	extra := ""
	if custom, ok := config.FindCustom(m.cfg.Providers.Customs, name); ok {
		extra = custom.APIKeyEnv
	} else if endpoint, ok := m.cfg.Providers.Endpoints[name]; ok {
		extra = endpoint.APIKeyEnv
	}
	return auth.CredentialsAvailable(name, m.store, extra)
}

func (m Model) keysAuth(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.authCancel != nil {
		switch {
		case msg.Code == tea.KeyEsc || keyText(msg) == "c":
			m.authCancel()
			m.authCancel = nil
			m.authCtx = nil
			m.authAttempt++ // Ignore the canceled command's eventual result.
			m.pending = nil
			m.authCode = nil
			m.input = inputNone
			m.authFailed = false
			m.status = ""
			m = m.leaveAuthView()
		}
		return m, nil
	}
	if m.authFailed {
		switch {
		case keyText(msg) == "r":
			return m.startLogin(m.authProv, m.authMethod)
		case msg.Code == tea.KeyEsc || keyText(msg) == "c":
			m.authFailed = false
			m.authCode = nil
			m.status = ""
			m = m.leaveAuthView()
		}
		return m, nil
	}
	switch {
	case msg.Code == tea.KeyEsc:
		m = m.leaveAuthView()
		return m, nil
	case msg.Code == tea.KeyDown || keyText(msg) == "j":
		if m.authCursor < len(m.authList)-1 {
			m.authCursor++
		}
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		if m.authCursor > 0 {
			m.authCursor--
		}
	case keyText(msg) == "x":
		if m.store != nil && m.authCursor >= 0 && m.authCursor < len(m.authList) {
			provider := m.authList[m.authCursor]
			if provider == "gitlab" || provider == "github" {
				target, _ := auth.PublicTarget(provider)
				_ = m.store.DeletePlatform(context.Background(), target)
			} else {
				_ = m.store.Delete(provider)
			}
			m.status = "logged out of " + provider
		}
	case keyText(msg) == "d":
		if provider := m.authList[m.authCursor]; provider == "xai" || provider == "gitlab" || provider == "github" {
			return m.startLogin(provider, "device")
		}
	case keyText(msg) == "k":
		if provider := m.authList[m.authCursor]; provider == "gitlab" || provider == "github" {
			m.authProv = provider
			m.input = inputAPIKey
			m.editBuf = ""
			m.status = "paste " + provider + " personal access token, enter to save"
			return m, nil
		}
	case msg.Code == tea.KeyEnter:
		prov := m.authList[m.authCursor]
		switch prov {
		case "github":
			return m.startLogin(prov, "device")
		case "openai", "xai", "gitlab":
			return m.startLogin(prov, "oauth")
		default:
			m.authProv = prov
			m.input = inputAPIKey
			m.editBuf = ""
			m.status = "paste " + prov + " API key, enter to save"
			return m, nil
		}
	}
	return m, nil
}

func (m Model) keysConfig(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		if m.onboardingRequired() {
			m.view = ViewOnboarding
		} else {
			m.view = ViewDashboard
		}
		return m, nil
	case msg.Code == tea.KeyTab || msg.Code == tea.KeyDown || keyText(msg) == "j":
		m.cfgField = (m.cfgField + 1) % 3
		return m, nil
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		m.cfgField = (m.cfgField + 2) % 3
		return m, nil
	case msg.Code == tea.KeyEnter:
		m.input = inputConfig
		return m, nil
	case keyText(msg) == "s":
		return m.saveConfig()
	}
	if keyText(msg) != "" && msg.Code != tea.KeySpace {
		m.input = inputConfig
		m.appendConfigField(keyText(msg))
	}
	return m, nil
}

func (m Model) keysOnboarding(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	choices := m.onboardingChoices()
	if m.onboardingStep == onboardingPlatform {
		choices = []string{"github", "gitlab"}
	}
	if len(choices) == 0 {
		m.status = "no supported AI providers are configured"
		return m, nil
	}
	switch {
	case msg.Code == tea.KeyDown || keyText(msg) == "j":
		m.onboardingCursor = (m.onboardingCursor + 1) % len(choices)
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		m.onboardingCursor = (m.onboardingCursor + len(choices) - 1) % len(choices)
	case msg.Code == tea.KeyEnter:
		if m.onboardingStep == onboardingProvider {
			m.onboardingProvider = choices[m.onboardingCursor]
			return m.finishProviderOnboarding()
		}
		m.onboardingPlatform = choices[m.onboardingCursor]
		return m.finishPlatformOnboarding()
	}
	return m, nil
}

func (m Model) finishProviderOnboarding() (tea.Model, tea.Cmd) {
	provider := config.CanonicalProviderID(m.onboardingProvider)
	extraEnv := ""
	if custom, ok := config.FindCustom(m.cfg.Providers.Customs, provider); ok {
		extraEnv = custom.APIKeyEnv
	}
	if _, ok := auth.CredentialFingerprint(provider, m.store, extraEnv); !ok {
		m.onboardingStep = onboardingProvider
		m.input = inputOnboardingSecret
		m.status = "enter an API key for " + provider
		return m, nil
	}
	m.onboardingProvider = provider
	m.onboardingStep = onboardingPlatform
	m.onboardingCursor = 0
	m.status = "provider credential selected"
	return m, nil
}

func (m Model) finishPlatformOnboarding() (tea.Model, tea.Cmd) {
	target, err := m.onboardingPlatformTarget()
	if err != nil {
		m.status = "platform configuration is invalid; repair it in config"
		return m, nil
	}
	fingerprint, ok := auth.PlatformCredentialFingerprint(target, m.store)
	if !ok {
		m.onboardingStep = onboardingSecret
		m.input = inputOnboardingSecret
		m.status = "enter a personal access token for " + m.onboardingPlatform
		return m, nil
	}
	providerFingerprint, ok := m.providerFingerprint()
	if !ok {
		m.onboardingStep = onboardingProvider
		m.status = "provider credential is missing; select it again"
		return m, nil
	}
	now := time.Now().UTC()
	next := m.cfg
	next.Onboarding = config.OnboardingState{
		SchemaVersion: config.OnboardingSchemaVersion, Provider: m.onboardingProvider,
		ProviderFingerprint: providerFingerprint, ProviderValidatedAt: now,
		Platform: m.onboardingPlatform, PlatformOrigin: target.Origin, PlatformAPIBase: target.APIBase,
		PlatformFingerprint: fingerprint, PlatformValidatedAt: now,
	}
	if err := m.saveSettings(next); err != nil {
		m.status = "could not save onboarding configuration; try again"
		return m, nil
	}
	m.cfg = next
	m.provider = m.onboardingProvider
	m.model = provider.DefaultModel(m.provider)
	m.view = ViewDashboard
	m.status = "onboarding complete"
	return m, nil
}

func (m Model) providerFingerprint() (string, bool) {
	extraEnv := ""
	if custom, ok := config.FindCustom(m.cfg.Providers.Customs, m.onboardingProvider); ok {
		extraEnv = custom.APIKeyEnv
	}
	return auth.CredentialFingerprint(m.onboardingProvider, m.store, extraEnv)
}

func (m Model) onboardingPlatformTarget() (auth.PlatformTarget, error) {
	switch m.onboardingPlatform {
	case "github":
		return auth.NewPlatformTarget("github", "https://github.com", m.cfg.GitHubAPI)
	case "gitlab":
		return auth.NewPlatformTarget("gitlab", m.cfg.GitLabURL, strings.TrimRight(m.cfg.GitLabURL, "/")+"/api/v4")
	default:
		return auth.PlatformTarget{}, fmt.Errorf("unsupported platform")
	}
}

func (m Model) onboardingChoices() []string {
	seen := map[string]bool{}
	var choices []string
	for _, name := range m.cfg.ProviderNames() {
		name = config.CanonicalProviderID(name)
		if name == "echo" || name == "ollama" || seen[name] {
			continue
		}
		seen[name] = true
		choices = append(choices, name)
	}
	return choices
}

func (m Model) saveSettings(next config.Settings) error {
	if m.saveFn != nil {
		return m.saveFn(next)
	}
	return config.Save(next)
}

func (m Model) onboardingRequired() bool {
	return m.store != nil && !m.cfg.OnboardingStatus(m.store).Complete
}

func (m Model) saveConfig() (tea.Model, tea.Cmd) {
	next := m.cfg
	next.GitHubAPI = strings.TrimRight(strings.TrimSpace(m.cfgGitHub), "/")
	next.GitLabURL = strings.TrimRight(strings.TrimSpace(m.cfgGitLab), "/")
	next.AnthropicURL = strings.TrimRight(strings.TrimSpace(m.cfgAnthropic), "/")
	if m.saveFn == nil {
		if err := config.Save(next); err != nil {
			m.status = err.Error()
			return m, nil
		}
	} else if err := m.saveFn(next); err != nil {
		m.status = err.Error()
		return m, nil
	}
	m.cfg = next
	m.status = "saved"
	return m, nil
}

func (m *Model) appendConfigField(s string) {
	switch m.cfgField {
	case cfgFieldGitHub:
		m.cfgGitHub += s
	case cfgFieldGitLab:
		m.cfgGitLab += s
	case cfgFieldAnthropic:
		m.cfgAnthropic += s
	}
}

func (m *Model) trimConfigField() {
	switch m.cfgField {
	case cfgFieldGitHub:
		m.cfgGitHub = trimLast(m.cfgGitHub)
	case cfgFieldGitLab:
		m.cfgGitLab = trimLast(m.cfgGitLab)
	case cfgFieldAnthropic:
		m.cfgAnthropic = trimLast(m.cfgAnthropic)
	}
}

func (m Model) waitOAuth(prov string, pending *auth.PendingLogin) tea.Cmd {
	ctx := m.authCtx
	if ctx == nil {
		ctx = context.Background()
	}
	attempt := m.authAttempt
	store := m.store
	return func() tea.Msg {
		msg, err := FinishOAuth(ctx, store, prov, pending)
		return authDoneMsg{message: msg, err: err, attempt: attempt}
	}
}

func (m Model) startLogin(prov, method string) (tea.Model, tea.Cmd) {
	m.authProv = prov
	m.authMethod = method
	m.authFailed = false
	m.authCode = nil
	m.pending = nil
	if method == "device" {
		m.authAttempt++
		attempt := m.authAttempt
		ctx, cancel := context.WithCancel(context.Background())
		m.authCtx, m.authCancel = ctx, cancel
		cfg := m.device
		if prov == "gitlab" {
			var err error
			cfg, err = auth.GitLabDeviceFlow(auth.GitLabOAuthClientID())
			if err != nil {
				m.authCancel = nil
				m.authCtx = nil
				m.authFailed = true
				m.status = err.Error()
				return m, nil
			}
		} else if prov == "github" {
			var err error
			cfg, err = auth.GitHubDeviceFlow(auth.GitHubOAuthClientID())
			if err != nil {
				m.authCancel = nil
				m.authCtx = nil
				m.authFailed = true
				m.status = err.Error()
				return m, nil
			}
		} else if cfg.DeviceURL == "" {
			cfg = auth.XAIDeviceFlow()
		}
		return m, func() tea.Msg {
			code, err := cfg.RequestCode(ctx)
			if err != nil {
				return authDoneMsg{err: err, attempt: attempt}
			}
			return deviceCodeMsg{code: code, cfg: cfg, provider: prov, attempt: attempt}
		}
	}
	if method == "oauth" && (prov == "openai" || prov == "xai" || prov == "gitlab") {
		m.authAttempt++
		m.authCtx, m.authCancel = context.WithCancel(context.Background())
		var (
			p   *auth.PendingLogin
			err error
		)
		if m.beginOAuth != nil {
			p, err = m.beginOAuth(prov)
		} else {
			flow := auth.OpenAIFlow()
			if prov == "xai" {
				flow = auth.XAIFlow()
			} else if prov == "gitlab" {
				flow, err = auth.GitLabFlow(auth.GitLabOAuthClientID())
				if err != nil {
					m.authCancel = nil
					m.authCtx = nil
					m.authFailed = true
					m.status = err.Error()
					return m, nil
				}
			}
			p, err = flow.Begin()
		}
		if err != nil {
			m.authCancel = nil
			m.authCtx = nil
			m.authFailed = true
			m.status = err.Error()
			return m, nil
		}
		m.pending = p
		m.status = "Waiting for browser authorization..."
		if !p.LoopbackListening() {
			m.input = inputOAuthPaste
			m.status = "could not bind loopback; paste the callback URL"
			return m, nil
		}
		return m, m.waitOAuth(prov, p)
	}
	if m.loginFn != nil {
		return m, func() tea.Msg {
			msg, err := m.loginFn(prov, method, "")
			return authDoneMsg{message: msg, err: err}
		}
	}
	return m, nil
}

func (m *Model) cycleProvider(delta int) {
	names := m.cfg.ProviderNames()
	if len(names) == 0 {
		names = []string{"anthropic", "openai", "xai", "google", "kimi", "deepseek", "echo"}
	}
	i := 0
	for j, n := range names {
		if n == m.provider {
			i = j
			break
		}
	}
	i = (i + delta + len(names)) % len(names)
	m.provider = names[i]
	m.models = nil
	m.modelsAvailable = false
	m.model = ""
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func (m *Model) toggleFocus() {
	opts := []string{"bugs", "style", "best-practices", "security", "performance"}
	// cycle through adding/removing current? toggle all via index 0 for tests: toggle first missing or first present
	// Space toggles the next option after last in focus, or first.
	have := map[string]bool{}
	for _, f := range m.focus {
		have[f] = true
	}
	for _, o := range opts {
		if !have[o] {
			m.focus = append(m.focus, o)
			return
		}
	}
	// all on — drop last
	if len(m.focus) > 1 {
		m.focus = m.focus[:len(m.focus)-1]
	}
}

func keyText(msg tea.KeyPressMsg) string {
	if msg.Text != "" {
		return msg.Text
	}
	if msg.Code >= 32 && msg.Code < 127 {
		return string(rune(msg.Code))
	}
	return ""
}

func trimLast(s string) string {
	if s == "" {
		return s
	}
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

// Accessors for tests.
func (m Model) URL() string       { return m.url }
func (m Model) Provider() string  { return m.provider }
func (m Model) ModelID() string   { return m.model }
func (m Model) Focus() []string   { return m.focus }
func (m Model) MaxComments() int  { return m.maxC }
func (m Model) Summary() string   { return m.summary }
func (m Model) CommentCount() int { return len(m.comments) }
func (m Model) ApprovedCount() int {
	n := 0
	for _, c := range m.comments {
		if c.Approved {
			n++
		}
	}
	return n
}
func (m Model) PostedCount() int        { return m.postedN }
func (m Model) Error() string           { return m.err }
func (m Model) Status() string          { return m.status }
func (m Model) ConfigGitHubAPI() string { return m.cfgGitHub }
func (m Model) ConfigGitLabURL() string { return m.cfgGitLab }
func (m Model) ConfigAnthropic() string { return m.cfgAnthropic }
func (m Model) CommentBody(i int) string {
	if i < 0 || i >= len(m.comments) {
		return ""
	}
	return m.comments[i].Body
}
func (m Model) CommentApproved(i int) bool {
	if i < 0 || i >= len(m.comments) {
		return false
	}
	return m.comments[i].Approved
}
