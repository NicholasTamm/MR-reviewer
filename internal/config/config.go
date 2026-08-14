package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/platform"
	"github.com/jonathanung/mr-reviewer/internal/provider"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

type Settings struct {
	GitLabURL           string
	GitHubAPI           string
	AllowInsecureGitLab bool
	Provider            string
	Model               string
	Focus               []string
	MaxComments         int
}

func Load() Settings {
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
	return Settings{
		GitLabURL:           strings.TrimRight(first(os.Getenv("MR_REVIEWER_GITLAB_URL"), "https://gitlab.com"), "/"),
		GitHubAPI:           strings.TrimRight(first(os.Getenv("MR_REVIEWER_GITHUB_API"), "https://api.github.com"), "/"),
		AllowInsecureGitLab: truthy(os.Getenv("MR_REVIEWER_ALLOW_INSECURE_GITLAB")),
		Provider:            first(os.Getenv("MR_REVIEWER_PROVIDER"), "anthropic"),
		Model:               os.Getenv("MR_REVIEWER_MODEL"),
		Focus:               focus,
		MaxComments:         maxC,
	}
}

func (s Settings) PlatformFor(info review.Info, store *auth.Store) (review.Platform, error) {
	switch info.Platform {
	case "github":
		tok, err := auth.PlatformToken("github", store)
		if err != nil {
			return nil, err
		}
		return &platform.GitHub{BaseURL: s.GitHubAPI, Token: tok}, nil
	case "gitlab":
		tok, err := auth.PlatformToken("gitlab", store)
		if err != nil {
			return nil, err
		}
		host := info.BaseURL
		if host == "" {
			host = s.GitLabURL
		}
		if strings.HasPrefix(host, "http://") && !s.AllowInsecureGitLab {
			return nil, fmt.Errorf("refusing to send GITLAB_TOKEN to an insecure HTTP GitLab URL; use HTTPS or set MR_REVIEWER_ALLOW_INSECURE_GITLAB=true")
		}
		return &platform.GitLab{BaseURL: host, Token: tok}, nil
	default:
		return nil, fmt.Errorf("unknown platform %q", info.Platform)
	}
}

func (s Settings) GitLabBrowser(store *auth.Store) (*platform.GitLab, error) {
	tok, err := auth.PlatformToken("gitlab", store)
	if err != nil {
		return nil, err
	}
	return &platform.GitLab{BaseURL: s.GitLabURL, Token: tok}, nil
}

func (s Settings) NewProvider(name, model string, store *auth.Store) (review.Provider, error) {
	if name == "" {
		name = s.Provider
	}
	if model == "" {
		model = s.Model
	}
	return provider.New(name, model, store, provider.Options{})
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
