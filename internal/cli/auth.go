package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/provider"
)

const authUsage = `Manage provider credentials.

Usage:
  mr-reviewer auth login <anthropic|openai|xai|google|kimi|deepseek|gitlab|github> [--api-key] [--device]
  mr-reviewer auth status
  mr-reviewer auth logout <provider>

Login methods:
  openai      OAuth "Sign in with ChatGPT" (default), or --api-key
  xai         OAuth browser flow (default), --device for headless, or --api-key
  anthropic   --api-key
  google      --api-key (alias: gemini)
  kimi        --api-key
  deepseek    --api-key
  gitlab      --api-key (GITLAB_TOKEN)
  github      --api-key (GITHUB_TOKEN)

Env vars take precedence: ANTHROPIC_API_KEY, OPENAI_API_KEY, XAI_API_KEY,
GEMINI_API_KEY / GOOGLE_API_KEY, KIMI_API_KEY, DEEPSEEK_API_KEY,
GITLAB_TOKEN, GITHUB_TOKEN.
`

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
		if err := runAuthLogin(store, args[1:], stdout, stderr); err != nil {
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
		if err := store.Delete(prov); err != nil {
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

func runAuthStatus(store *auth.Store, stdout io.Writer) {
	names := append(auth.BuiltinProviders(), "gitlab", "github")
	for _, n := range names {
		fmt.Fprintf(stdout, "%-10s %s\n", n, auth.Describe(n, store))
	}
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

func runAuthLogin(store *auth.Store, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mr-reviewer auth login <provider> [--api-key] [--device]")
	}
	prov := auth.CanonicalProvider(args[0])
	useAPIKey := hasFlag(args[1:], "--api-key")
	useDevice := hasFlag(args[1:], "--device")
	ctx := context.Background()

	switch prov {
	case "anthropic", "google", "kimi", "deepseek", "gitlab", "github":
		return loginAPIKey(store, prov, stdout)
	case "openai":
		if useAPIKey {
			return loginAPIKey(store, prov, stdout)
		}
		return loginOAuth(ctx, store, "openai", auth.OpenAIFlow(), stdout)
	case "xai":
		if useAPIKey {
			return loginAPIKey(store, prov, stdout)
		}
		if useDevice {
			return loginXAIDevice(ctx, store, stdout)
		}
		return loginOAuth(ctx, store, "xai", auth.XAIFlow(), stdout)
	default:
		return fmt.Errorf("unknown provider %q (want %s)", prov, strings.Join(append(provider.Names(), "gitlab", "github"), ", "))
	}
}

func loginAPIKey(store *auth.Store, prov string, stdout io.Writer) error {
	fmt.Fprintf(stdout, "Paste your %s API key: ", prov)
	key, err := readSecret()
	if err != nil {
		return err
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

func loginXAIDevice(ctx context.Context, store *auth.Store, stdout io.Writer) error {
	code, err := auth.XAIDeviceFlow().RequestCode(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Visit %s and enter code %s\n", code.VerificationURI, code.UserCode)
	if code.VerificationURIComplete != "" {
		fmt.Fprintln(stdout, code.VerificationURIComplete)
	}
	tokens, err := auth.XAIDeviceFlow().Poll(ctx, code)
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

func readSecret() (string, error) {
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", nil
	}
	return strings.TrimSpace(sc.Text()), nil
}
