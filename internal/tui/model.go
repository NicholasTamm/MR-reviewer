package tui

import (
	"context"
	"strings"
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
	inputSearch
	inputURL
	inputModel
	inputEditComment
	inputEditSummary
	inputAPIKey
	inputOAuthPaste
	inputConfig
)

type configField int

const (
	cfgFieldGitHub configField = iota
	cfgFieldGitLab
	cfgFieldAnthropic
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

	// dashboard
	search     string
	groups     []review.ProjectMergeRequests
	flat       []dashItem
	cursor     int
	dashLoaded bool

	// configure / link
	url      string
	provider string
	model    string
	focus    []string
	maxC     int
	autoPost bool
	field    field

	// HITL
	summary   string
	comments  []hitlComment
	meta      review.Metadata
	postedN   int
	editBuf   string
	input     inputMode
	reviewing bool

	// auth
	authList   []string
	authCursor int
	pending    *auth.PendingLogin
	authProv   string

	// injectable for tests / wiring
	loadDash   func(search string) ([]review.ProjectMergeRequests, error)
	runReview  func(url, provider, model string, focus []string, maxC int) (review.Result, review.Metadata, error)
	postFn     func(result review.Result) error
	loginFn    func(provider, method, secret string) (string, error)
	beginOAuth func(provider string) (*auth.PendingLogin, error)
	device     auth.DeviceConfig
	saveFn     func(config.Settings) error

	// config panel
	cfgField     configField
	cfgGitHub    string
	cfgGitLab    string
	cfgAnthropic string
}

type dashItem struct {
	header bool
	path   string
	mr     review.MergeRequestSummary
}

type dashMsg struct {
	groups []review.ProjectMergeRequests
	err    error
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
}

type deviceCodeMsg struct {
	code *auth.DeviceCode
	cfg  auth.DeviceConfig
}

type Deps struct {
	Store        *auth.Store
	Settings     config.Settings
	LoadDash     func(search string) ([]review.ProjectMergeRequests, error)
	RunReview    func(url, provider, model string, focus []string, maxC int) (review.Result, review.Metadata, error)
	Post         func(result review.Result) error
	Login        func(provider, method, secret string) (string, error)
	BeginOAuth   func(provider string) (*auth.PendingLogin, error)
	Device       auth.DeviceConfig
	StartView    View
	SaveSettings func(config.Settings) error
}

func New(deps Deps) Model {
	cfg := deps.Settings
	if cfg.Provider == "" {
		cfg = config.Load()
	}
	m := Model{
		width:        80,
		height:       24,
		view:         ViewDashboard,
		cfg:          cfg,
		store:        deps.Store,
		provider:     cfg.Provider,
		model:        cfg.Model,
		focus:        append([]string{}, cfg.Focus...),
		maxC:         cfg.MaxComments,
		loadDash:     deps.LoadDash,
		runReview:    deps.RunReview,
		postFn:       deps.Post,
		loginFn:      deps.Login,
		beginOAuth:   deps.BeginOAuth,
		device:       deps.Device,
		saveFn:       deps.SaveSettings,
		cfgGitHub:    cfg.GitHubAPI,
		cfgGitLab:    cfg.GitLabURL,
		cfgAnthropic: cfg.AnthropicURL,
		authList:     append(auth.BuiltinProviders(), "gitlab", "github"),
	}
	if deps.StartView == ViewConfig {
		m.view = ViewConfig
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
	return m.fetchDash()
}

func (m Model) ViewName() View { return m.view }

func (m Model) fetchDash() tea.Cmd {
	if m.loadDash == nil {
		return nil
	}
	search := m.search
	return func() tea.Msg {
		g, err := m.loadDash(search)
		return dashMsg{groups: g, err: err}
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
	case dashMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.dashLoaded = true
			return m, nil
		}
		m.groups = msg.groups
		m.rebuildFlat()
		m.dashLoaded = true
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
		m.status = devicePrompt(msg.code)
		store := m.store
		cfg := msg.cfg
		code := msg.code
		return m, func() tea.Msg {
			ctx := context.Background()
			tok, err := cfg.Poll(ctx, code)
			if err != nil {
				return authDoneMsg{err: err}
			}
			out, err := PersistLogin(ctx, store, "xai", "device", tok, "")
			return authDoneMsg{message: out, err: err}
		}
	case authDoneMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = msg.message
		m.pending = nil
		m.input = inputNone
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
		m.url += s
	case inputSearch:
		m.search += s
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
	default:
		if m.view == ViewLink && m.field == fieldURL {
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
		m.input = inputNone
		m.editBuf = ""
		return m, nil
	case tea.KeyEnter:
		return m.commitInput()
	case tea.KeyBackspace:
		switch m.input {
		case inputSearch:
			m.search = trimLast(m.search)
		case inputURL:
			m.url = trimLast(m.url)
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
	case inputSearch:
		m.search += ch
	case inputURL:
		m.url += ch
	case inputModel:
		m.model += ch
	case inputConfig:
		m.appendConfigField(ch)
	default:
		m.editBuf += ch
	}
	return m, nil
}

func (m Model) commitInput() (tea.Model, tea.Cmd) {
	switch m.input {
	case inputSearch:
		m.input = inputNone
		m.cursor = 0
		return m, m.fetchDash()
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
	}
	m.input = inputNone
	return m, nil
}

func (m Model) keysDashboard(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc || keyText(msg) == "q":
		return m, tea.Quit
	case keyText(msg) == "/" || keyText(msg) == "s":
		m.input = inputSearch
		return m, nil
	case keyText(msg) == "l":
		m.view = ViewLink
		m.field = fieldURL
		return m, nil
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
	case msg.Code == tea.KeyDown || keyText(msg) == "j":
		if m.cursor < len(m.flat)-1 {
			m.cursor++
			if m.flat[m.cursor].header && m.cursor < len(m.flat)-1 {
				m.cursor++
			}
		}
	case msg.Code == tea.KeyUp || keyText(msg) == "k":
		if m.cursor > 0 {
			m.cursor--
			if m.flat[m.cursor].header && m.cursor > 0 {
				m.cursor--
			}
		}
	case msg.Code == tea.KeyEnter:
		if m.cursor >= 0 && m.cursor < len(m.flat) && !m.flat[m.cursor].header {
			m.url = m.flat[m.cursor].mr.WebURL
			m.view = ViewLink
			m.field = fieldURL
			return m, nil
		}
	}
	return m, nil
}

func (m Model) keysLink(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		m.view = ViewDashboard
		return m, nil
	case msg.Code == tea.KeyTab:
		m.field = (m.field + 1) % 6
		return m, nil
	case msg.Code == tea.KeyEnter:
		if m.field == fieldURL && m.url == "" {
			m.input = inputURL
			return m, nil
		}
		if m.field == fieldModel {
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
		if m.field == fieldProvider {
			m.cycleProvider(-1)
		}
		return m, nil
	case msg.Code == tea.KeyRight:
		if m.field == fieldProvider {
			m.cycleProvider(1)
		}
		return m, nil
	}
	if m.field == fieldURL && keyText(msg) != "" && msg.Code != tea.KeySpace {
		m.url += keyText(msg)
	}
	if m.field == fieldModel && keyText(msg) != "" && msg.Code != tea.KeySpace {
		m.input = inputModel
		m.model += keyText(msg)
	}
	return m, nil
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
			return m, m.fetchDash()
		}
	case "q":
		return m, tea.Quit
	}
	if msg.Code == tea.KeyEnter {
		m.view = ViewDashboard
		m.url = ""
		m.comments = nil
		return m, m.fetchDash()
	}
	return m, nil
}

func (m Model) keysAuth(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEsc:
		m.view = ViewDashboard
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
			_ = m.store.Delete(m.authList[m.authCursor])
			m.status = "logged out of " + m.authList[m.authCursor]
		}
	case keyText(msg) == "d":
		if m.authList[m.authCursor] == "xai" {
			return m.startLogin("xai", "device")
		}
	case msg.Code == tea.KeyEnter:
		prov := m.authList[m.authCursor]
		switch prov {
		case "openai", "xai":
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
		m.view = ViewDashboard
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
	store := m.store
	return func() tea.Msg {
		msg, err := FinishOAuth(context.Background(), store, prov, pending)
		return authDoneMsg{message: msg, err: err}
	}
}

func (m Model) startLogin(prov, method string) (tea.Model, tea.Cmd) {
	m.authProv = prov
	if method == "device" {
		cfg := m.device
		if cfg.DeviceURL == "" {
			cfg = auth.XAIDeviceFlow()
		}
		return m, func() tea.Msg {
			code, err := cfg.RequestCode(context.Background())
			if err != nil {
				return authDoneMsg{err: err}
			}
			return deviceCodeMsg{code: code, cfg: cfg}
		}
	}
	if method == "oauth" && (prov == "openai" || prov == "xai") {
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
			}
			p, err = flow.Begin()
		}
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.pending = p
		m.status = "browser login started"
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
	m.model = provider.DefaultModel(m.provider)
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

func (m *Model) rebuildFlat() {
	m.flat = m.flat[:0]
	for _, g := range m.groups {
		m.flat = append(m.flat, dashItem{header: true, path: g.ProjectPath})
		for _, mr := range g.MergeRequests {
			m.flat = append(m.flat, dashItem{mr: mr, path: g.ProjectPath})
		}
	}
	if m.cursor >= len(m.flat) {
		m.cursor = 0
	}
	if len(m.flat) > 0 && m.flat[m.cursor].header && m.cursor < len(m.flat)-1 {
		m.cursor++
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
func (m Model) Search() string          { return m.search }
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
