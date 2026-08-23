package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/platform"
	"github.com/jonathanung/mr-reviewer/internal/provider"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

type Settings struct {
	GitLabURL           string
	GitHubAPI           string
	AnthropicURL        string
	AllowInsecureGitLab bool
	Provider            string
	Model               string
	Focus               []string
	MaxComments         int
	Parallel            bool
	ParallelThreshold   int
	Providers           ProvidersFile
	Onboarding          OnboardingState
}

const OnboardingSchemaVersion = 1

// OnboardingState is client-independent setup metadata. Credentials stay in
// auth.Store or the environment and must never be added here.
type OnboardingState struct {
	SchemaVersion       int       `json:"schemaVersion,omitempty"`
	Provider            string    `json:"provider,omitempty"`
	ProviderFingerprint string    `json:"providerFingerprint,omitempty"`
	Platform            string    `json:"platform,omitempty"`
	PlatformOrigin      string    `json:"platformOrigin,omitempty"`
	PlatformAPIBase     string    `json:"platformAPIBase,omitempty"`
	PlatformFingerprint string    `json:"platformFingerprint,omitempty"`
	ProviderValidatedAt time.Time `json:"providerValidatedAt,omitempty"`
	PlatformValidatedAt time.Time `json:"platformValidatedAt,omitempty"`
}

type OnboardingStatus struct {
	Complete bool
	Reason   string
}

// fileSettings is the persisted subset of Settings.
type fileSettings struct {
	GitHubAPI    string           `json:"githubAPI,omitempty"`
	GitLabURL    string           `json:"gitlabURL,omitempty"`
	AnthropicURL string           `json:"anthropicURL,omitempty"`
	Onboarding   *OnboardingState `json:"onboarding,omitempty"`
}

func Load() Settings {
	file := readFileSettings()
	pf := LoadProviders()
	onboarding := OnboardingState{}
	if file.Onboarding != nil {
		onboarding = *file.Onboarding
	}

	githubAPI := "https://api.github.com"
	if file.GitHubAPI != "" {
		githubAPI = file.GitHubAPI
	}
	if v := os.Getenv("MR_REVIEWER_GITHUB_API"); v != "" {
		githubAPI = v
	}

	gitlabURL := "https://gitlab.com"
	if file.GitLabURL != "" {
		gitlabURL = file.GitLabURL
	}
	if v := os.Getenv("MR_REVIEWER_GITLAB_URL"); v != "" {
		gitlabURL = v
	}

	anthropicURL := ""
	if ep, ok := pf.Endpoints["anthropic"]; ok {
		anthropicURL = ep.BaseURL
	}
	if file.AnthropicURL != "" {
		anthropicURL = file.AnthropicURL
	}
	if v := os.Getenv("MR_REVIEWER_ANTHROPIC_URL"); v != "" {
		anthropicURL = v
	}

	focus := review.DefaultFocus
	if v := os.Getenv("MR_REVIEWER_FOCUS"); v != "" {
		focus = splitCSV(v)
	}
	maxC := 10
	if v := os.Getenv("MR_REVIEWER_MAX_COMMENTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxC = n
		}
	}
	threshold := 10
	if v := os.Getenv("MR_REVIEWER_PARALLEL_THRESHOLD"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threshold = n
		}
	}
	return Settings{
		GitLabURL:           strings.TrimRight(gitlabURL, "/"),
		GitHubAPI:           strings.TrimRight(githubAPI, "/"),
		AnthropicURL:        strings.TrimRight(anthropicURL, "/"),
		AllowInsecureGitLab: truthy(os.Getenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB")),
		Provider:            first(os.Getenv("MR_REVIEWER_PROVIDER"), "anthropic"),
		Model:               os.Getenv("MR_REVIEWER_MODEL"),
		Focus:               focus,
		MaxComments:         maxC,
		Parallel:            truthy(os.Getenv("MR_REVIEWER_PARALLEL")),
		ParallelThreshold:   threshold,
		Providers:           pf,
		Onboarding:          onboarding,
	}
}

func readFileSettings() fileSettings {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return fileSettings{}
	}
	var fs fileSettings
	if err := json.Unmarshal(data, &fs); err != nil {
		return fileSettings{}
	}
	return fs
}

// Save persists GitHub API, GitLab, and Anthropic URLs. Mode 0600.
func Save(s Settings) error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var onboarding *OnboardingState
	if s.Onboarding != (OnboardingState{}) {
		state := s.Onboarding
		onboarding = &state
	}
	payload := fileSettings{
		GitHubAPI:    strings.TrimRight(strings.TrimSpace(s.GitHubAPI), "/"),
		GitLabURL:    strings.TrimRight(strings.TrimSpace(s.GitLabURL), "/"),
		AnthropicURL: strings.TrimRight(strings.TrimSpace(s.AnthropicURL), "/"),
		Onboarding:   onboarding,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SaveOnboarding persists shared onboarding metadata without exposing secrets.
func SaveOnboarding(state OnboardingState) error {
	settings := Load()
	settings.Onboarding = state
	return Save(settings)
}

// OnboardingStatus evaluates whether a client may enable reviews. Clients must
// validate each integration before recording its corresponding timestamp.
func (s Settings) OnboardingStatus(store *auth.Store) OnboardingStatus {
	state := s.Onboarding
	if state.SchemaVersion != OnboardingSchemaVersion {
		return OnboardingStatus{Reason: "onboarding configuration is missing or out of date"}
	}
	provider := CanonicalProviderID(state.Provider)
	if !s.onboardingProvider(provider) {
		return OnboardingStatus{Reason: "select one supported AI provider"}
	}
	if state.ProviderValidatedAt.IsZero() {
		return OnboardingStatus{Reason: "validate the selected AI provider"}
	}
	custom, isCustom := FindCustom(s.Providers.Customs, provider)
	extraEnv := ""
	if isCustom {
		extraEnv = custom.APIKeyEnv
	}
	providerFingerprint, ok := auth.CredentialFingerprint(provider, store, extraEnv)
	if !ok || state.ProviderFingerprint == "" || state.ProviderFingerprint != providerFingerprint {
		return OnboardingStatus{Reason: "AI provider credentials are missing, expired, or changed"}
	}
	platform := strings.ToLower(strings.TrimSpace(state.Platform))
	if platform != "github" && platform != "gitlab" {
		return OnboardingStatus{Reason: "select one supported Git platform"}
	}
	if state.PlatformValidatedAt.IsZero() {
		return OnboardingStatus{Reason: "validate the selected Git platform"}
	}
	target, err := auth.NewPlatformTarget(platform, state.PlatformOrigin, state.PlatformAPIBase)
	platformFingerprint, ok := auth.PlatformCredentialFingerprint(target, store)
	if err != nil || !ok || state.PlatformFingerprint == "" || state.PlatformFingerprint != platformFingerprint {
		return OnboardingStatus{Reason: "Git platform credentials are missing, expired, or changed"}
	}
	return OnboardingStatus{Complete: true}
}

func (s Settings) onboardingProvider(name string) bool {
	if name == "" || name == "echo" || name == "ollama" {
		return false
	}
	if _, ok := BuiltinProviderNames[name]; ok {
		return true
	}
	_, ok := FindCustom(s.Providers.Customs, name)
	return ok
}

func (s Settings) PlatformFor(info review.Info, store *auth.Store) (review.Platform, error) {
	switch info.Platform {
	case "github":
		origin := info.BaseURL
		if origin == "" {
			origin = "https://github.com"
		}
		target, err := auth.NewPlatformTarget("github", origin, s.GitHubAPI)
		if err != nil {
			return nil, err
		}
		return &platform.GitHub{BaseURL: target.APIBase, Credentials: auth.PlatformCredentialSource(target, store)}, nil
	case "gitlab":
		host := info.BaseURL
		if host == "" {
			host = s.GitLabURL
		}
		if strings.HasPrefix(host, "http://") && !s.AllowInsecureGitLab {
			return nil, fmt.Errorf("refusing to send GITLAB_TOKEN to an insecure HTTP GitLab URL; use HTTPS or set MR_REVIEWER_ALLOW_INSECURE_GITLAB=true")
		}
		target, err := auth.NewPlatformTarget("gitlab", host, strings.TrimRight(host, "/")+"/api/v4")
		if err != nil {
			return nil, err
		}
		return &platform.GitLab{BaseURL: target.Origin, Credentials: auth.PlatformCredentialSource(target, store)}, nil
	default:
		return nil, fmt.Errorf("unknown platform %q", info.Platform)
	}
}

func (s Settings) GitLabBrowser(store *auth.Store) (*platform.GitLab, error) {
	target, err := auth.NewPlatformTarget("gitlab", s.GitLabURL, strings.TrimRight(s.GitLabURL, "/")+"/api/v4")
	if err != nil {
		return nil, err
	}
	return &platform.GitLab{BaseURL: target.Origin, Credentials: auth.PlatformCredentialSource(target, store)}, nil
}

func (s Settings) GitHubBrowser(store *auth.Store) (*platform.GitHub, error) {
	origin := "https://github.com"
	if s.Onboarding.Platform == "github" && s.Onboarding.PlatformOrigin != "" {
		origin = s.Onboarding.PlatformOrigin
	}
	target, err := auth.NewPlatformTarget("github", origin, s.GitHubAPI)
	if err != nil {
		return nil, err
	}
	return &platform.GitHub{BaseURL: target.APIBase, Credentials: auth.PlatformCredentialSource(target, store)}, nil
}

func (s Settings) NewProvider(name, model string, store *auth.Store) (review.Provider, error) {
	if name == "" {
		name = s.Provider
	}
	if model == "" {
		model = s.Model
	}
	name = CanonicalProviderID(name)
	if c, ok := FindCustom(s.Providers.Customs, name); ok {
		if model == "" && len(c.Models) > 0 {
			model = c.Models[0]
		}
		return provider.NewCustom(c.Name, model, c.BaseURL, string(c.API), store, c.APIKeyEnv)
	}
	opts := provider.Options{AnthropicURL: s.AnthropicURL}
	if ep, ok := s.Providers.Endpoints[name]; ok {
		switch name {
		case "anthropic":
			if opts.AnthropicURL == "" {
				opts.AnthropicURL = ep.BaseURL
			}
		case "openai":
			opts.OpenAIURL = ep.BaseURL
		case "xai":
			opts.XAIURL = ep.BaseURL
		case "google":
			opts.GoogleURL = ep.BaseURL
		case "kimi":
			opts.KimiURL = ep.BaseURL
		case "deepseek":
			opts.DeepSeekURL = ep.BaseURL
		case "ollama":
			opts.OllamaURL = ep.BaseURL
		}
	}
	return provider.New(name, model, store, opts)
}

func (s Settings) ProviderNames() []string {
	names := append([]string{}, provider.Names()...)
	for _, c := range s.Providers.Customs {
		names = append(names, c.Name)
	}
	return names
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
