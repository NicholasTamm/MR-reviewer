package api

import (
	"context"
	"net/http"

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
	customModels := []string(nil)
	if custom, ok := config.FindCustom(settings.Providers.Customs, name); ok {
		customModels = custom.Models
	}
	key := ""
	if name == "openai" {
		var err error
		key, err = catalogKey(ctx, name, settings, store)
		if err != nil {
			return provider.Unavailable(name, err.Error())
		}
	} else if name == "ollama" || auth.CredentialsAvailable(name, store, extraEnv(settings, name)) {
		var err error
		key, err = catalogKey(ctx, name, settings, store)
		if err != nil && name != "ollama" {
			key = ""
		}
	}
	return provider.DiscoverOne(ctx, client, provider.DiscoverQuery{
		Name: name, BaseURL: catalogBaseURL(settings, name), Key: key, CustomModels: customModels,
	})
}

func extraEnv(settings config.Settings, name string) string {
	if custom, ok := config.FindCustom(settings.Providers.Customs, name); ok {
		return custom.APIKeyEnv
	}
	if ep, ok := settings.Providers.Endpoints[name]; ok {
		return ep.APIKeyEnv
	}
	return ""
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
