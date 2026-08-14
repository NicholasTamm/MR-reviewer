package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

// Options override endpoints (tests) and Ollama host.
type Options struct {
	AnthropicURL string
	OpenAIURL    string
	XAIURL       string
	GoogleURL    string
	KimiURL      string
	DeepSeekURL  string
	OllamaURL    string
}

func emptyKey(context.Context) (string, error) { return "", nil }

// New builds a review.Provider for a strike-set (or echo/ollama) name.
func New(name, model string, store *auth.Store, opts Options) (review.Provider, error) {
	name = Canonical(strings.ToLower(strings.TrimSpace(name)))
	if model == "" {
		model = DefaultModel(name)
	}
	switch name {
	case "echo":
		return Echo{}, nil
	case "anthropic":
		url := first(opts.AnthropicURL, os.Getenv("MR_REVIEWER_ANTHROPIC_URL"), BaseURL("anthropic"))
		return &Anthropic{BaseURL: url, Model: model, Key: auth.BearerSource("anthropic", store)}, nil
	case "openai":
		url := first(opts.OpenAIURL, os.Getenv("MR_REVIEWER_OPENAI_URL"), BaseURL("openai"))
		return &OpenAICompat{NameID: "openai", BaseURL: url, Model: model, Key: auth.BearerSource("openai", store)}, nil
	case "xai":
		url := first(opts.XAIURL, os.Getenv("MR_REVIEWER_XAI_URL"), BaseURL("xai"))
		return &OpenAICompat{NameID: "xai", BaseURL: url, Model: model, Key: auth.BearerSource("xai", store)}, nil
	case "kimi":
		url := first(opts.KimiURL, os.Getenv("MR_REVIEWER_KIMI_URL"), BaseURL("kimi"))
		return &OpenAICompat{NameID: "kimi", BaseURL: url, Model: model, Key: auth.BearerSource("kimi", store)}, nil
	case "deepseek":
		url := first(opts.DeepSeekURL, os.Getenv("MR_REVIEWER_DEEPSEEK_URL"), BaseURL("deepseek"))
		return &OpenAICompat{NameID: "deepseek", BaseURL: url, Model: model, Key: auth.BearerSource("deepseek", store)}, nil
	case "ollama":
		host := first(opts.OllamaURL, os.Getenv("OLLAMA_HOST"), "http://localhost:11434")
		if !strings.HasSuffix(host, "/v1") {
			host = strings.TrimRight(host, "/") + "/v1"
		}
		return &OpenAICompat{NameID: "ollama", BaseURL: host, Model: model, Key: emptyKey}, nil
	case "google":
		url := first(opts.GoogleURL, os.Getenv("MR_REVIEWER_GOOGLE_URL"), BaseURL("google"))
		return &Google{BaseURL: url, Model: model, Key: auth.BearerSource("google", store)}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q (want anthropic, openai, xai, google, kimi, deepseek, echo, ollama; gemini is an alias of google)", name)
	}
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
