package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/platform"
	"github.com/jonathanung/mr-reviewer/internal/provider"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

var corsOrigins = map[string]struct{}{
	"http://localhost:5173": {},
	"http://localhost:3000": {},
	"http://localhost:8080": {},
	"null":                  {},
}

// Server is the Electron-compatible review HTTP API.
type Server struct {
	Settings config.Settings
	Store    *auth.Store
	Token    string
	Jobs     *jobStore
	Now      func() time.Time
	NewID    func() string

	NewPlatform func(info review.Info) (review.Platform, error)
	NewProvider func(name, model string) (review.Provider, error)
	GitLab      func() (gitlabSurface, error)
	GitHub      func() (platform.Catalog, error)
	Discover    func(ctx context.Context) []provider.Models
	HTTP        *http.Client

	mu             sync.Mutex
	sessions       *authSessionStore
	SaveOnboarding func(config.OnboardingState) error
	DeviceFlow     func(name string) (auth.DeviceConfig, error)
	BeginOAuth     func(name string) (*auth.PendingLogin, error)
}

type gitlabSurface interface {
	ListVisibleMergeRequests(ctx context.Context, search string) ([]review.ProjectMergeRequests, error)
	ListVisibleProjects(ctx context.Context, search string) ([]review.ProjectSummary, error)
	ListProjectMergeRequests(ctx context.Context, projectID int) (review.ProjectMergeRequests, error)
}

// New constructs a production server from shared config and credentials.
func New(settings config.Settings, store *auth.Store, token string) *Server {
	s := &Server{
		Settings: settings,
		Store:    store,
		Token:    token,
		Jobs:     newJobStore(),
		Now:      func() time.Time { return time.Now().UTC() },
		NewID:    newID,
		sessions: newAuthSessionStore(),
	}
	s.NewPlatform = func(info review.Info) (review.Platform, error) {
		return settings.PlatformFor(info, store)
	}
	s.NewProvider = func(name, model string) (review.Provider, error) {
		return settings.NewProvider(name, model, store)
	}
	s.GitLab = func() (gitlabSurface, error) {
		return settings.GitLabBrowser(store)
	}
	s.GitHub = func() (platform.Catalog, error) {
		return settings.GitHubBrowser(store)
	}
	s.Discover = func(ctx context.Context) []provider.Models {
		return discoverModels(ctx, settings, store, s.HTTP)
	}
	return s
}

// Handler returns the Electron REST surface.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/reviews", s.handleSubmit)
	mux.HandleFunc("GET /api/reviews/{job_id}", s.handleJobStatus)
	mux.HandleFunc("GET /api/reviews/{job_id}/results", s.handleResults)
	mux.HandleFunc("PATCH /api/reviews/{job_id}/comments/{comment_id}", s.handleEditComment)
	mux.HandleFunc("POST /api/reviews/{job_id}/post", s.handlePost)
	mux.HandleFunc("GET /api/config/defaults", s.handleConfigDefaults)
	mux.HandleFunc("GET /api/providers/models", s.handleProviderModels)
	mux.HandleFunc("GET /api/gitlab/merge-requests", s.handleGitLabMergeRequests)
	mux.HandleFunc("GET /api/gitlab/projects", s.handleGitLabProjects)
	mux.HandleFunc("GET /api/gitlab/projects/{project_id}/merge-requests", s.handleGitLabProjectMRs)
	mux.HandleFunc("GET /api/github/projects", s.handleGitHubProjects)
	mux.HandleFunc("GET /api/github/projects/{owner}/{repo}/pull-requests", s.handleGitHubProjectPRs)
	mux.HandleFunc("GET /api/onboarding", s.handleOnboardingStatus)
	mux.HandleFunc("POST /api/onboarding", s.handleOnboardingComplete)
	mux.HandleFunc("POST /api/onboarding/secret", s.handleOnboardingSecret)
	mux.HandleFunc("POST /api/auth/sessions", s.handleAuthStart)
	mux.HandleFunc("GET /api/auth/sessions/{session_id}", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/sessions/{session_id}/cancel", s.handleAuthCancel)
	mux.HandleFunc("POST /api/auth/sessions/{session_id}/paste", s.handleAuthPaste)
	return s.middleware(mux)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "null"
		}
		if _, ok := corsOrigins[origin]; ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "*")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		if s.Token != "" && r.URL.Path != "/api/health" {
			want := "Bearer " + s.Token
			got := r.Header.Get("Authorization")
			if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
				writeDetail(w, http.StatusForbidden, "Unauthorized")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeDetail(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Server) id() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return newID()
}

func classifyError(err error) string {
	if err == nil {
		return "unknown"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "insecure http gitlab"), strings.Contains(msg, "is not set"), strings.Contains(msg, "no credentials"):
		return "config"
	case strings.Contains(msg, "invalid url"), strings.Contains(msg, "unsupported url"), strings.Contains(msg, "could not parse"), strings.Contains(msg, "not a gitlab"), strings.Contains(msg, "must not include credentials"):
		return "invalid_url"
	}
	authish := strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "authentication") || strings.Contains(msg, "forbidden")
	switch {
	case strings.Contains(msg, "gitlab") || strings.Contains(msg, "github") || strings.Contains(msg, "platform"):
		if authish {
			return "platform_auth"
		}
		return "platform"
	case strings.Contains(msg, "provider") || strings.Contains(msg, "anthropic") || strings.Contains(msg, "openai") || strings.Contains(msg, "google") || strings.Contains(msg, "ollama"):
		if authish {
			return "provider_auth"
		}
		return "provider"
	case authish:
		return "platform_auth"
	default:
		return "unknown"
	}
}
