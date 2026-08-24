package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/platform"
	"github.com/jonathanung/mr-reviewer/internal/provider"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

type liveSession struct {
	cfg    config.Settings
	store  *auth.Store
	plat   review.Platform
	info   review.Info
	device auth.DeviceConfig
}

func (s *liveSession) catalog(name string) (platform.Catalog, error) {
	client, err := s.cfg.PlatformFor(review.Info{Platform: name}, s.store)
	if err != nil {
		return nil, fmt.Errorf("%s browsing unavailable: %w", name, err)
	}
	// Fail before issuing a catalog request when this platform has no usable credential.
	switch client := client.(type) {
	case *platform.GitHub:
		_, err = client.Credentials(context.Background())
	case *platform.GitLab:
		_, err = client.Credentials(context.Background())
	}
	if err != nil {
		return nil, fmt.Errorf("%s browsing unavailable: %w", name, err)
	}
	catalog, ok := client.(platform.Catalog)
	if !ok {
		return nil, fmt.Errorf("%s browsing unavailable: not supported", name)
	}
	return catalog, nil
}

func (s *liveSession) loadProjects(name string) ([]review.Project, error) {
	catalog, err := s.catalog(name)
	if err != nil {
		return nil, err
	}
	projects, err := catalog.ListProjects(context.Background(), "")
	if err != nil {
		return nil, fmt.Errorf("%s project catalog: %w", name, err)
	}
	return projects, nil
}

func (s *liveSession) loadReviews(name string, project review.Project) ([]review.ReviewSummary, error) {
	catalog, err := s.catalog(name)
	if err != nil {
		return nil, err
	}
	reviews, err := catalog.ListProjectReviews(context.Background(), project, "")
	if err != nil {
		return nil, fmt.Errorf("%s project %s reviews: %w", name, project.Path, err)
	}
	return reviews, nil
}

func (s *liveSession) runReview(url, name, model string, focus []string, maxC int) (review.Result, review.Metadata, error) {
	info, err := review.Parse(url)
	if err != nil {
		return review.Result{}, review.Metadata{}, err
	}
	plat, err := s.cfg.PlatformFor(info, s.store)
	if err != nil {
		return review.Result{}, review.Metadata{}, err
	}
	prov, err := s.cfg.NewProvider(name, model, s.store)
	if err != nil {
		return review.Result{}, review.Metadata{}, err
	}
	res, err := review.Run(context.Background(), review.Options{
		URL: url, Provider: prov, Platform: plat, Focus: focus, MaxComments: maxC, DryRun: true,
	})
	if err != nil {
		return review.Result{}, review.Metadata{}, err
	}
	s.plat = plat
	s.info = info
	return res, res.Meta, nil
}

func (s *liveSession) post(result review.Result) error {
	if s.plat == nil {
		return fmt.Errorf("nothing to post — run a review first")
	}
	return s.plat.PostReview(context.Background(), s.info, result, nil)
}

func (s *liveSession) login(provider, method, secret string) (string, error) {
	ctx := context.Background()
	switch method {
	case "key":
		return PersistLogin(ctx, s.store, provider, "key", nil, secret)
	case "device":
		cfg := s.device
		if cfg.DeviceURL == "" {
			cfg = auth.XAIDeviceFlow()
		}
		return RunDeviceLogin(ctx, s.store, cfg)
	case "oauth":
		// Browser/device OAuth must persist full Tokens via FinishOAuth /
		// PersistLogin — an access string is not enough for OpenAI exchange.
		return "", fmt.Errorf("oauth for %s requires FinishOAuth with full tokens", provider)
	default:
		return "", fmt.Errorf("unknown login method %q", method)
	}
}

// Run starts the Bubble Tea 2 TUI.
func Run() error {
	return RunWith(ViewDashboard)
}

// RunWith starts the TUI on the given view (ViewConfig for --config).
func RunWith(start View) error {
	store, err := auth.OpenStore(auth.DefaultPath())
	if err != nil {
		return err
	}
	cfg := config.Load()
	sess := &liveSession{cfg: cfg, store: store}
	m := New(Deps{
		Store:           store,
		Settings:        cfg,
		LoadProjects:    sess.loadProjects,
		LoadReviews:     sess.loadReviews,
		RunReview:       sess.runReview,
		Post:            sess.post,
		Login:           sess.login,
		StartView:       start,
		SaveSettings:    sess.saveSettings,
		CheckOnboarding: true,
		LoadModels:      sess.loadModels,
	})
	_, err = tea.NewProgram(m).Run()
	return err
}

func (s *liveSession) loadModels(name string) ([]string, error) {
	extra := ""
	if custom, ok := config.FindCustom(s.cfg.Providers.Customs, name); ok {
		extra = custom.APIKeyEnv
	}
	key := ""
	if name == "ollama" || auth.CredentialsAvailable(name, s.store, extra) {
		if name != "ollama" {
			got, err := auth.BearerSourceEnv(name, s.store, extra)(context.Background())
			if err == nil {
				key = got
			}
		}
	}
	customModels := []string(nil)
	if custom, ok := config.FindCustom(s.cfg.Providers.Customs, name); ok {
		customModels = custom.Models
	}
	base := provider.BaseURL(name)
	if custom, ok := config.FindCustom(s.cfg.Providers.Customs, name); ok && custom.BaseURL != "" {
		base = custom.BaseURL
	}
	got := provider.DiscoverOne(context.Background(), nil, provider.DiscoverQuery{
		Name: name, BaseURL: base, Key: key, CustomModels: customModels,
	})
	return got.Models, nil
}

func (s *liveSession) saveSettings(st config.Settings) error {
	if err := config.Save(st); err != nil {
		return err
	}
	s.cfg = config.Load()
	return nil
}
