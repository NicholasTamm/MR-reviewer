package auth

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLegacyPlatformPATMigrationIsPublicCloudOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	if err := os.WriteFile(path, []byte(`{"gitlab":{"type":"api","apiKey":"glpat-secret"},"github":{"type":"api","apiKey":"ghp-secret"},"openai":{"type":"api","apiKey":"provider-key"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"gitlab", "github"} {
		target, _ := PublicTarget(platform)
		credential, ok := store.GetPlatform(target)
		if !ok || credential.Type != PlatformPAT || credential.Token == "" {
			t.Fatalf("%s migration = %+v ok=%v", platform, credential, ok)
		}
	}
	if _, ok := store.Get("gitlab"); ok {
		t.Fatal("legacy GitLab record was retained")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var disk map[string]json.RawMessage
	if err := json.Unmarshal(data, &disk); err != nil {
		t.Fatal(err)
	}
	if _, ok := disk["gitlab"]; ok {
		t.Fatal("legacy GitLab record persisted")
	}
	if _, ok := disk["platforms"]; !ok {
		t.Fatal("platform credentials not persisted")
	}
}

func TestPlatformCredentialRejectsTargetMismatchAndRedactsSecrets(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := PublicTarget("github")
	if err := store.SetPlatform(context.Background(), public, PlatformCredential{Type: PlatformPAT, Token: "ghp-very-secret"}); err != nil {
		t.Fatal(err)
	}
	other, _ := NewPlatformTarget("github", "https://ghe.example", "https://ghe.example/api/v3")
	_, err = ResolvePlatformCredential(context.Background(), other, store)
	if !errors.Is(err, ErrPlatformLoginRequired) {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), "ghp-very-secret") {
		t.Fatalf("secret leaked in %q", err)
	}
}

func TestPublicCloudEnvironmentPATDoesNotApplyToOtherTargets(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-secret")
	public, _ := PublicTarget("github")
	credential, err := ResolvePlatformCredential(context.Background(), public, nil)
	if err != nil || credential.Type != PlatformPAT || credential.Token != "env-secret" {
		t.Fatalf("public credential = %+v err=%v", credential, err)
	}
	other, _ := NewPlatformTarget("github", "https://ghe.example", "https://ghe.example/api/v3")
	_, err = ResolvePlatformCredential(context.Background(), other, nil)
	if !errors.Is(err, ErrPlatformLoginRequired) {
		t.Fatalf("other target error = %v", err)
	}
}

func TestPlatformStoreCancelledAndFailedWriteDoNotChangeCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	target, _ := PublicTarget("gitlab")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := store.SetPlatform(ctx, target, PlatformCredential{Type: PlatformPAT, Token: "one"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled write = %v", err)
	}
	if _, ok := store.GetPlatform(target); ok {
		t.Fatal("cancelled write changed memory")
	}

	badParent := filepath.Join(t.TempDir(), "parent")
	if err := os.Mkdir(badParent, 0o700); err != nil {
		t.Fatal(err)
	}
	broken, err := OpenStore(filepath.Join(badParent, "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(badParent); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badParent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := broken.SetPlatform(context.Background(), target, PlatformCredential{Type: PlatformPAT, Token: "two"}); err == nil {
		t.Fatal("expected failed write")
	}
	if _, ok := broken.GetPlatform(target); ok {
		t.Fatal("failed write changed memory")
	}
}

func TestPlatformOAuthRefreshIsSingleFlightAndFailsWithoutSecrets(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	target, _ := PublicTarget("gitlab")
	if err := store.SetPlatform(context.Background(), target, PlatformCredential{Type: PlatformOAuth, Token: "old-access", Refresh: "refresh-secret", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	start := make(chan struct{})
	store.SetPlatformRefresher(func(context.Context, PlatformTarget, PlatformCredential) (PlatformCredential, error) {
		calls.Add(1)
		<-start
		return PlatformCredential{Type: PlatformOAuth, Token: "new-access", Refresh: "new-refresh", ExpiresAt: time.Now().Add(time.Hour)}, nil
	})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			credential, err := ResolvePlatformCredential(context.Background(), target, store)
			if err != nil || credential.Token != "new-access" {
				errs <- err
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("refresh result = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("refresh calls = %d", calls.Load())
	}

	if err := store.SetPlatform(context.Background(), target, PlatformCredential{Type: PlatformOAuth, Token: "old-access", Refresh: "refresh-secret", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	store.SetPlatformRefresher(func(context.Context, PlatformTarget, PlatformCredential) (PlatformCredential, error) {
		return PlatformCredential{}, errors.New("refresh-secret rejected")
	})
	_, err = ResolvePlatformCredential(context.Background(), target, store)
	if !errors.Is(err, ErrPlatformLoginRequired) || strings.Contains(err.Error(), "refresh-secret") {
		t.Fatalf("refresh failure = %v", err)
	}
}

func TestGitLabOAuthRefreshUsesOnlyItsStoredClientID(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	target, _ := PublicTarget("gitlab")
	if err := store.SetPlatform(context.Background(), target, PlatformCredential{Type: PlatformOAuth, Token: "expired", Refresh: "refresh", ClientID: "client-id", ExpiresAt: time.Now().Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	var gotTarget PlatformTarget
	var gotCredential PlatformCredential
	store.SetPlatformRefresher(func(_ context.Context, target PlatformTarget, credential PlatformCredential) (PlatformCredential, error) {
		gotTarget, gotCredential = target, credential
		return PlatformCredential{Type: PlatformOAuth, Token: "fresh", Refresh: "next", ClientID: credential.ClientID, ExpiresAt: time.Now().Add(time.Hour)}, nil
	})
	credential, err := ResolvePlatformCredential(context.Background(), target, store)
	if err != nil || credential.Token != "fresh" || gotTarget != target || gotCredential.ClientID != "client-id" {
		t.Fatalf("credential=%+v target=%+v supplied=%+v err=%v", credential, gotTarget, gotCredential, err)
	}
}

func TestNonExpiringPlatformOAuthDoesNotAttemptRefresh(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	target, _ := PublicTarget("github")
	if err := store.SetPlatform(context.Background(), target, PlatformCredential{Type: PlatformOAuth, Token: "long-lived", ClientID: "client-id"}); err != nil {
		t.Fatal(err)
	}
	store.SetPlatformRefresher(func(context.Context, PlatformTarget, PlatformCredential) (PlatformCredential, error) {
		t.Fatal("non-expiring OAuth credentials must not refresh")
		return PlatformCredential{}, nil
	})
	credential, err := ResolvePlatformCredential(context.Background(), target, store)
	if err != nil || credential.Token != "long-lived" {
		t.Fatalf("credential=%+v err=%v", credential, err)
	}
}

func TestGitHubOAuthLoginIsPublicTargetBoundAndRequiresScopeAndClientID(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	public, _ := PublicTarget("github")
	if _, err := CompletePlatformLogin(context.Background(), store, public, "", &Tokens{Access: "access", Scope: "repo"}); err == nil || !strings.Contains(err.Error(), "client ID") {
		t.Fatalf("missing client ID error = %v", err)
	}
	if _, err := CompletePlatformLogin(context.Background(), store, public, "client-id", &Tokens{Access: "access", Scope: "public_repo"}); err == nil || !strings.Contains(err.Error(), "repo scope") {
		t.Fatalf("scope error = %v", err)
	}
	enterprise, _ := NewPlatformTarget("github", "https://ghe.example", "https://ghe.example/api/v3")
	if _, err := CompletePlatformLogin(context.Background(), store, enterprise, "client-id", &Tokens{Access: "access", Scope: "repo"}); err == nil || !strings.Contains(err.Error(), "GitHub.com only") {
		t.Fatalf("enterprise error = %v", err)
	}
	if _, err := CompletePlatformLogin(context.Background(), store, public, "client-id", &Tokens{Access: "access", Scope: "repo"}); err != nil {
		t.Fatal(err)
	}
	credential, ok := store.GetPlatform(public)
	if !ok || credential.ClientID != "client-id" || credential.Type != PlatformOAuth {
		t.Fatalf("credential = %+v ok=%v", credential, ok)
	}
}
