package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

type liveSession struct {
	cfg    config.Settings
	store  *auth.Store
	plat   review.Platform
	info   review.Info
	device auth.DeviceConfig
}

func (s *liveSession) loadDash(search string) ([]review.ProjectMergeRequests, error) {
	gl, err := s.cfg.GitLabBrowser(s.store)
	if err != nil {
		return nil, err
	}
	return gl.ListVisibleMergeRequests(context.Background(), search)
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
	store, err := auth.OpenStore(auth.DefaultPath())
	if err != nil {
		return err
	}
	cfg := config.Load()
	sess := &liveSession{cfg: cfg, store: store}
	m := New(Deps{
		Store:     store,
		Settings:  cfg,
		LoadDash:  sess.loadDash,
		RunReview: sess.runReview,
		Post:      sess.post,
		Login:     sess.login,
	})
	_, err = tea.NewProgram(m).Run()
	return err
}
