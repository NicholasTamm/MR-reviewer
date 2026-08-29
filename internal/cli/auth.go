package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/jonathanung/mr-reviewer/internal/auth"
)

const authUsage = `Manage provider credentials.

Usage:
  mr-reviewer auth login <anthropic|openai|xai|google|kimi|deepseek|gitlab|github> [--api-key [TOKEN]] [--pat [TOKEN]] [--device] [--client-id ID]
  mr-reviewer auth status
  mr-reviewer auth logout <provider>

Login methods:
  openai      OAuth "Sign in with ChatGPT" (default), or --api-key to paste a key
  xai         OAuth browser flow (default), --device for headless machines,
              or --api-key to paste a key
  anthropic   paste a key (OAuth not wired)
  google      paste a Google AI Studio key (alias: gemini)
  kimi        paste a key
  deepseek    paste a key
  gitlab      browser OAuth with PKCE, --device for headless login, or --pat for a PAT
  github      device OAuth, or --pat for a PAT

--api-key with no value prompts (hidden on a TTY). Use --pat for GitLab or
GitHub personal access tokens. --api-key remains accepted for existing scripts.

Env vars take precedence over stored credentials:
  ANTHROPIC_API_KEY, OPENAI_API_KEY, XAI_API_KEY,
  GEMINI_API_KEY / GOOGLE_API_KEY, KIMI_API_KEY, DEEPSEEK_API_KEY,
  GITLAB_TOKEN, GITHUB_TOKEN. OAuth client IDs may be overridden with
  GITLAB_OAUTH_CLIENT_ID or GITHUB_OAUTH_CLIENT_ID.

GitLab and GitHub OAuth use built-in public client IDs. Do not provide or store
an application secret. --client-id remains available to use a custom OAuth app.
`

// promptSecret is the strike-shaped secret reader. Tests replace it.
var promptSecret = promptSecretDefault

func RunAuth(args []string, stdout, stderr io.Writer) int {
	store, err := auth.OpenStore(auth.DefaultPath())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(args) == 0 {
		fmt.Fprint(stdout, authUsage)
		return 0
	}
	switch args[0] {
	case "login":
		if err := runAuthLogin(store, args[1:], stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "status":
		runAuthStatus(store, stdout)
	case "logout":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: mr-reviewer auth logout <provider>")
			return 1
		}
		prov := auth.CanonicalProvider(args[1])
		var err error
		if prov == "gitlab" || prov == "github" {
			target, targetErr := auth.PublicTarget(prov)
			if targetErr != nil {
				err = targetErr
			} else {
				err = store.DeletePlatform(context.Background(), target)
			}
		} else {
			err = store.Delete(prov)
		}
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, "Logged out of", prov)
	default:
		fmt.Fprint(stdout, authUsage)
		return 1
	}
	return 0
}

var builtinAuthProviders = []string{"anthropic", "openai", "xai", "google", "kimi", "deepseek", "gitlab", "github"}

func runAuthStatus(store *auth.Store, stdout io.Writer) {
	stored := store.Providers()
	for _, name := range builtinAuthProviders {
		status := "not logged in"
		if key, ok := envKey(name); ok {
			status = "using " + key + " from environment"
		} else if name == "gitlab" || name == "github" {
			target, _ := auth.PublicTarget(name)
			if cred, ok := store.GetPlatform(target); ok {
				if cred.Type == auth.PlatformPAT {
					status = "personal access token stored"
				} else if !cred.ExpiresAt.IsZero() && time.Now().After(cred.ExpiresAt) {
					status = "OAuth (access token expired; will refresh on use)"
				} else {
					status = "OAuth"
				}
			}
		} else if cred, ok := store.Get(name); ok {
			switch {
			case cred.Type == auth.TypeAPIKey:
				status = "API key stored"
			case cred.APIKey != "":
				status = "OAuth + exchanged API key"
			case !cred.ExpiresAt.IsZero() && time.Now().After(cred.ExpiresAt):
				status = "OAuth (access token expired; will refresh on use)"
			default:
				status = "OAuth"
			}
		}
		fmt.Fprintf(stdout, "  %-10s %s\n", name, status)
	}
	for _, p := range stored {
		if !slices.Contains(builtinAuthProviders, p) {
			fmt.Fprintf(stdout, "  %-10s credential stored (unknown provider)\n", p)
		}
	}
}

func envKey(provider string) (string, bool) {
	for _, name := range auth.EnvNames(provider) {
		if name != "" && os.Getenv(name) != "" {
			return name, true
		}
	}
	return "", false
}

func runAuthLogin(store *auth.Store, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mr-reviewer auth login <anthropic|openai|xai|google|kimi|deepseek|gitlab|github> [--api-key [TOKEN]] [--device]")
	}
	prov := auth.CanonicalProvider(args[0])
	useAPIKey, keyValue := parseAPIKeyFlag(args[1:])
	useDevice := hasFlag(args[1:], "--device")
	clientID := flagValue(args[1:], "--client-id")
	ctx := context.Background()

	switch prov {
	case "github":
		if useAPIKey {
			return loginPlatformPAT(store, prov, keyValue, stdout)
		}
		if clientID == "" {
			clientID = auth.GitHubOAuthClientID()
		}
		return loginGitHubDevice(ctx, store, clientID, stdout)
	case "gitlab":
		if useAPIKey {
			return loginPlatformPAT(store, prov, keyValue, stdout)
		}
		if clientID == "" {
			clientID = auth.GitLabOAuthClientID()
		}
		if useDevice {
			return loginGitLabDevice(ctx, store, clientID, stdout)
		}
		return loginGitLabOAuth(ctx, store, clientID, stdout)
	case "anthropic", "google", "kimi", "deepseek":
		return loginAPIKey(store, prov, keyValue, stdout)
	case "openai":
		if useAPIKey {
			return loginAPIKey(store, prov, keyValue, stdout)
		}
		return loginOAuth(ctx, store, "openai", auth.OpenAIFlow(), stdout)
	case "xai":
		if useAPIKey {
			return loginAPIKey(store, prov, keyValue, stdout)
		}
		return loginXAIOAuth(ctx, store, useDevice, stdout)
	default:
		return fmt.Errorf("unknown provider %q (want anthropic, openai, xai, google, kimi, deepseek, gitlab, or github; gemini is an alias of google)", prov)
	}
}

func parseAPIKeyFlag(args []string) (used bool, value string) {
	for i, a := range args {
		if a == "--api-key" || a == "--pat" {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return true, args[i+1]
			}
			return true, ""
		}
		for _, flag := range []string{"--api-key=", "--pat="} {
			if rest, ok := strings.CutPrefix(a, flag); ok {
				return true, rest
			}
		}
	}
	return false, ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func flagValue(args []string, name string) string {
	for i, arg := range args {
		if arg == name && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			return args[i+1]
		}
		if value, ok := strings.CutPrefix(arg, name+"="); ok {
			return value
		}
	}
	return ""
}

func loginAPIKeyPrompt(prov string) string {
	switch auth.CanonicalProvider(prov) {
	case "google":
		return "Paste your Google AI Studio API key (aistudio.google.com/apikey): "
	case "gitlab":
		return "Paste your GitLab personal access token (api scope): "
	case "github":
		return "Paste your GitHub personal access token: "
	default:
		return fmt.Sprintf("Paste your %s API key: ", prov)
	}
}

func loginAPIKey(store *auth.Store, prov, keyValue string, stdout io.Writer) error {
	key := strings.TrimSpace(keyValue)
	if key == "" {
		var err error
		key, err = promptSecret(stdout, loginAPIKeyPrompt(prov))
		if err != nil {
			return err
		}
		key = strings.TrimSpace(key)
	}
	if key == "" {
		return fmt.Errorf("no key entered")
	}
	if err := store.Set(prov, auth.Credential{Type: auth.TypeAPIKey, APIKey: key}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Stored %s API key in %s\n", prov, store.Path())
	return nil
}

func loginPlatformPAT(store *auth.Store, prov, keyValue string, stdout io.Writer) error {
	key := strings.TrimSpace(keyValue)
	if key == "" {
		var err error
		key, err = promptSecret(stdout, loginAPIKeyPrompt(prov))
		if err != nil {
			return err
		}
		key = strings.TrimSpace(key)
	}
	if key == "" {
		return fmt.Errorf("no key entered")
	}
	target, err := auth.PublicTarget(prov)
	if err != nil {
		return err
	}
	if err := store.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: key}); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Stored %s personal access token in %s\n", prov, store.Path())
	return nil
}

func loginGitLabOAuth(ctx context.Context, store *auth.Store, clientID string, stdout io.Writer) error {
	flow, err := auth.GitLabFlow(clientID)
	if err != nil {
		return err
	}
	tokens, err := flow.Login(ctx)
	if err != nil {
		return err
	}
	target, _ := auth.PublicTarget("gitlab")
	msg, err := auth.CompletePlatformLogin(ctx, store, target, clientID, tokens)
	if err != nil {
		return err
	}
	fprintln(stdout, msg)
	return nil
}

func loginGitLabDevice(ctx context.Context, store *auth.Store, clientID string, stdout io.Writer) error {
	flow, err := auth.GitLabDeviceFlow(clientID)
	if err != nil {
		return err
	}
	code, err := flow.RequestCode(ctx)
	if err != nil {
		return fmt.Errorf("GitLab device authorization unavailable; use browser PKCE login instead: %w", err)
	}
	fprintf(stdout, "Open %s on any device and enter code: %s\nWaiting for authorization…\n", code.VerificationURI, code.UserCode)
	tokens, err := flow.Poll(ctx, code)
	if err != nil {
		return err
	}
	target, _ := auth.PublicTarget("gitlab")
	msg, err := auth.CompletePlatformLogin(ctx, store, target, clientID, tokens)
	if err != nil {
		return err
	}
	fprintln(stdout, msg)
	return nil
}

func loginGitHubDevice(ctx context.Context, store *auth.Store, clientID string, stdout io.Writer) error {
	flow, err := auth.GitHubDeviceFlow(clientID)
	if err != nil {
		return err
	}
	code, err := flow.RequestCode(ctx)
	if err != nil {
		return err
	}
	fprintf(stdout, "Open %s on any device and enter code: %s\nWaiting for authorization…\n", code.VerificationURI, code.UserCode)
	tokens, err := flow.Poll(ctx, code)
	if err != nil {
		return err
	}
	target, _ := auth.PublicTarget("github")
	msg, err := auth.CompletePlatformLogin(ctx, store, target, clientID, tokens)
	if err != nil {
		return err
	}
	fprintln(stdout, msg)
	return nil
}

func fprintf(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }
func fprintln(w io.Writer, args ...any)               { _, _ = fmt.Fprintln(w, args...) }

func loginOAuth(ctx context.Context, store *auth.Store, prov string, flow auth.FlowConfig, stdout io.Writer) error {
	tokens, err := flow.Login(ctx)
	if err != nil {
		return err
	}
	msg, err := auth.CompleteLogin(ctx, store, prov, tokens)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, msg)
	return nil
}

func loginXAIOAuth(ctx context.Context, store *auth.Store, device bool, stdout io.Writer) error {
	var tokens *auth.Tokens
	var err error
	if device {
		flow := auth.XAIDeviceFlow()
		code, reqErr := flow.RequestCode(ctx)
		if reqErr != nil {
			return reqErr
		}
		fmt.Fprintf(stdout, "Open %s on any device and enter code: %s\n", code.VerificationURI, code.UserCode)
		if code.VerificationURIComplete != "" {
			fmt.Fprintln(stdout, "Or open directly:", code.VerificationURIComplete)
		}
		fmt.Fprintln(stdout, "Waiting for authorization…")
		tokens, err = flow.Poll(ctx, code)
	} else {
		tokens, err = auth.XAIFlow().Login(ctx)
	}
	if err != nil {
		return err
	}
	msg, err := auth.CompleteLogin(ctx, store, "xai", tokens)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, msg)
	return nil
}

func promptSecretDefault(output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		key, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(output)
		return strings.TrimSpace(string(key)), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(strings.TrimSpace(line)) == 0 {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
