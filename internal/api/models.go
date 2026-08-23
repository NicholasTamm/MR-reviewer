package api

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/provider"
)

func discoverModels(ctx context.Context, settings config.Settings, store *auth.Store, client *http.Client) []provider.Models {
	var out []provider.Models
	for _, name := range settings.ProviderNames() {
		out = append(out, discoverOne(ctx, settings, store, client, name))
	}
	return out
}

func discoverOne(ctx context.Context, settings config.Settings, store *auth.Store, client *http.Client, name string) provider.Models {
	name = config.CanonicalProviderID(name)
	if name == "echo" {
		return provider.Models{Provider: "echo", Models: []string{"echo"}, Available: true}
	}
	if custom, ok := config.FindCustom(settings.Providers.Customs, name); ok && len(custom.Models) > 0 {
		if !auth.CredentialsAvailable(name, store, custom.APIKeyEnv) {
			return provider.Unavailable(name, missingKeyMessage(name, custom.APIKeyEnv))
		}
		models := append([]string{}, custom.Models...)
		sort.Strings(models)
		return provider.Models{Provider: name, Models: models, Available: true}
	}
	key, err := catalogKey(ctx, name, settings, store)
	if err != nil {
		return provider.Unavailable(name, sanitizeModelError(err))
	}
	models, err := provider.ListRemoteModels(ctx, client, name, catalogBaseURL(settings, name), key)
	if err != nil {
		return provider.Unavailable(name, "Unable to retrieve available models.")
	}
	sort.Strings(models)
	return provider.Models{Provider: name, Models: models, Available: true}
}

func catalogKey(ctx context.Context, name string, settings config.Settings, store *auth.Store) (string, error) {
	if name == "ollama" {
		return "", nil
	}
	extra := ""
	if custom, ok := config.FindCustom(settings.Providers.Customs, name); ok {
		extra = custom.APIKeyEnv
	} else if ep, ok := settings.Providers.Endpoints[name]; ok {
		extra = ep.APIKeyEnv
	}
	if extra != "" {
		return auth.BearerSourceEnv(name, store, extra)(ctx)
	}
	return auth.BearerSource(name, store)(ctx)
}

func catalogBaseURL(settings config.Settings, name string) string {
	if custom, ok := config.FindCustom(settings.Providers.Customs, name); ok && custom.BaseURL != "" {
		return custom.BaseURL
	}
	if ep, ok := settings.Providers.Endpoints[name]; ok && ep.BaseURL != "" {
		return ep.BaseURL
	}
	if name == "anthropic" && settings.AnthropicURL != "" {
		return settings.AnthropicURL
	}
	return provider.BaseURL(name)
}

func missingKeyMessage(name, extraEnv string) string {
	if extraEnv != "" {
		return extraEnv + " is not set"
	}
	envs := auth.EnvNames(name)
	if len(envs) == 0 {
		return "API key is not set"
	}
	return envs[0] + " is not set"
}

func sanitizeModelError(err error) string {
	if err == nil {
		return "Unable to retrieve available models."
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "no credentials") || strings.Contains(msg, "is not set") {
		return msg
	}
	return "Unable to retrieve available models."
}
