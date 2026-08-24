package api

import (
	"net/http"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
)

func (s *Server) handleOnboardingStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.onboardingStatus())
}

func (s *Server) handleOnboardingSecret(w http.ResponseWriter, r *http.Request) {
	var req onboardingSecretRequest
	if err := decodeJSON(r, &req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	name := config.CanonicalProviderID(req.Name)
	secret := strings.TrimSpace(req.Secret)
	if secret == "" {
		writeDetail(w, http.StatusUnprocessableEntity, "enter a credential without sending it back in the response")
		return
	}
	if s.Store == nil {
		writeDetail(w, http.StatusInternalServerError, "shared credential store is unavailable")
		return
	}
	switch kind {
	case "provider":
		if !s.supportedOnboardingProvider(name) {
			writeDetail(w, http.StatusUnprocessableEntity, "select one supported AI provider")
			return
		}
		if err := s.Store.Set(name, auth.Credential{Type: auth.TypeAPIKey, APIKey: secret}); err != nil {
			writeDetail(w, http.StatusInternalServerError, "could not save provider credential")
			return
		}
	case "platform":
		if name != "github" && name != "gitlab" {
			writeDetail(w, http.StatusUnprocessableEntity, "select one supported Git platform")
			return
		}
		target, err := s.platformTarget(name)
		if err != nil {
			writeDetail(w, http.StatusUnprocessableEntity, "platform configuration is invalid")
			return
		}
		if err := s.Store.SetPlatform(r.Context(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: secret}); err != nil {
			writeDetail(w, http.StatusInternalServerError, "could not save platform credential")
			return
		}
	default:
		writeDetail(w, http.StatusUnprocessableEntity, "kind must be provider or platform")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "kind": kind, "name": name, "has_credential": true})
}

func (s *Server) handleOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	var req onboardingCompleteRequest
	if err := decodeJSON(r, &req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	provider := config.CanonicalProviderID(req.Provider)
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if !s.supportedOnboardingProvider(provider) {
		writeDetail(w, http.StatusUnprocessableEntity, "select one supported AI provider")
		return
	}
	if platform != "github" && platform != "gitlab" {
		writeDetail(w, http.StatusUnprocessableEntity, "select one supported Git platform")
		return
	}
	providerFingerprint, ok := s.providerFingerprint(provider)
	if !ok {
		writeDetail(w, http.StatusUnprocessableEntity, "validate the selected AI provider")
		return
	}
	target, err := s.platformTarget(platform)
	if err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "platform configuration is invalid")
		return
	}
	platformFingerprint, ok := auth.PlatformCredentialFingerprint(target, s.Store)
	if !ok {
		writeDetail(w, http.StatusUnprocessableEntity, "validate the selected Git platform")
		return
	}
	now := s.now()
	state := config.OnboardingState{
		SchemaVersion:       config.OnboardingSchemaVersion,
		Provider:            provider,
		ProviderFingerprint: providerFingerprint,
		ProviderValidatedAt: now,
		Platform:            platform,
		PlatformOrigin:      target.Origin,
		PlatformAPIBase:     target.APIBase,
		PlatformFingerprint: platformFingerprint,
		PlatformValidatedAt: now,
	}
	if err := s.saveOnboarding(state); err != nil {
		writeDetail(w, http.StatusInternalServerError, "could not save onboarding configuration")
		return
	}
	writeJSON(w, http.StatusOK, s.onboardingStatus())
}

func (s *Server) onboardingStatus() onboardingStatusJSON {
	status := s.Settings.OnboardingStatus(s.Store)
	state := s.Settings.Onboarding
	out := onboardingStatusJSON{
		Complete:         status.Complete,
		Reason:           status.Reason,
		Repair:           !status.Complete && state.SchemaVersion == config.OnboardingSchemaVersion,
		SelectedProvider: config.CanonicalProviderID(state.Provider),
		SelectedPlatform: strings.ToLower(strings.TrimSpace(state.Platform)),
		Providers:        s.onboardingProviders(),
		Platforms:        s.onboardingPlatforms(),
	}
	return out
}

func (s *Server) onboardingProviders() []onboardingOptionJSON {
	seen := map[string]bool{}
	var out []onboardingOptionJSON
	for _, name := range s.Settings.ProviderNames() {
		name = config.CanonicalProviderID(name)
		if !s.supportedOnboardingProvider(name) || seen[name] {
			continue
		}
		seen[name] = true
		_, ok := s.providerFingerprint(name)
		out = append(out, onboardingOptionJSON{ID: name, HasCredential: ok, Methods: providerAuthMethods(name)})
	}
	if out == nil {
		out = []onboardingOptionJSON{}
	}
	return out
}

func (s *Server) onboardingPlatforms() []onboardingOptionJSON {
	out := make([]onboardingOptionJSON, 0, 2)
	for _, name := range []string{"github", "gitlab"} {
		target, err := s.platformTarget(name)
		has := false
		if err == nil {
			_, has = auth.PlatformCredentialFingerprint(target, s.Store)
		}
		out = append(out, onboardingOptionJSON{ID: name, HasCredential: has, Methods: platformAuthMethods(name)})
	}
	return out
}

func (s *Server) supportedOnboardingProvider(name string) bool {
	return s.Settings.SupportsOnboardingProvider(name)
}

func (s *Server) providerFingerprint(name string) (string, bool) {
	extraEnv := ""
	if custom, ok := config.FindCustom(s.Settings.Providers.Customs, name); ok {
		extraEnv = custom.APIKeyEnv
	}
	return auth.CredentialFingerprint(name, s.Store, extraEnv)
}

func (s *Server) platformTarget(name string) (auth.PlatformTarget, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "github":
		return auth.NewPlatformTarget("github", "https://github.com", s.Settings.GitHubAPI)
	case "gitlab":
		host := s.Settings.GitLabURL
		if host == "" {
			host = "https://gitlab.com"
		}
		return auth.NewPlatformTarget("gitlab", host, strings.TrimRight(host, "/")+"/api/v4")
	default:
		return auth.PlatformTarget{}, errUnsupportedPlatform
	}
}

func (s *Server) saveOnboarding(state config.OnboardingState) error {
	save := s.SaveOnboarding
	if save == nil {
		save = config.SaveOnboarding
	}
	if err := save(state); err != nil {
		return err
	}
	s.mu.Lock()
	s.Settings.Onboarding = state
	s.mu.Unlock()
	return nil
}

func providerAuthMethods(name string) []string {
	switch name {
	case "openai":
		return []string{"oauth", "key"}
	case "xai":
		return []string{"oauth", "device", "key"}
	default:
		return []string{"key"}
	}
}

func platformAuthMethods(name string) []string {
	switch name {
	case "github":
		return []string{"device", "key"}
	case "gitlab":
		return []string{"oauth", "device", "key"}
	default:
		return []string{"key"}
	}
}
