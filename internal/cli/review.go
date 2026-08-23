package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
	"github.com/jonathanung/mr-reviewer/internal/review"
)

// ReviewArgs is the parsed headless review command.
type ReviewArgs struct {
	URL         string
	Provider    string
	Model       string
	Focus       []string
	MaxComments int
	DryRun      bool
	Post        bool
}

func ParseReviewArgs(args []string) (ReviewArgs, error) {
	fs := flag.NewFlagSet("review", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var (
		provider    string
		model       string
		focus       string
		maxComments int
		dryRun      bool
		noPost      bool
		post        bool
	)
	fs.StringVar(&provider, "provider", "", "model provider (anthropic, openai, xai, google, kimi, deepseek, echo)")
	fs.StringVar(&model, "model", "", "model id")
	fs.StringVar(&focus, "focus", "", "comma-separated focus areas")
	fs.IntVar(&maxComments, "max-comments", 0, "max non-error comments")
	fs.BoolVar(&dryRun, "dry-run", false, "review without posting")
	fs.BoolVar(&noPost, "no-post", false, "alias of --dry-run")
	fs.BoolVar(&post, "post", false, "post the review to the platform")
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			if eq := strings.Index(name, "="); eq >= 0 {
				name = name[:eq]
			}
			needsVal := name == "provider" || name == "model" || name == "focus" || name == "max-comments"
			if needsVal && !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	if err := fs.Parse(flags); err != nil {
		return ReviewArgs{}, err
	}
	url := ""
	if len(pos) > 0 {
		url = pos[0]
	}
	if url == "" {
		return ReviewArgs{}, fmt.Errorf("usage: mr-reviewer review <url> [--dry-run|--no-post] [--post] [--provider NAME] [--model ID]")
	}
	if post && (dryRun || noPost) {
		return ReviewArgs{}, fmt.Errorf("cannot combine --post with --dry-run/--no-post")
	}
	out := ReviewArgs{
		URL:         url,
		Provider:    provider,
		Model:       model,
		MaxComments: maxComments,
		DryRun:      !post,
		Post:        post,
	}
	if focus != "" {
		out.Focus = splitCSV(focus)
	}
	return out, nil
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

// RunReview executes the headless review and writes JSON to stdout.
func RunReview(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	parsed, err := ParseReviewArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		_ = json.NewEncoder(stdout).Encode(map[string]string{"error": err.Error()})
		return 1
	}
	store, err := auth.OpenStore(auth.DefaultPath())
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		_ = json.NewEncoder(stdout).Encode(map[string]string{"error": err.Error()})
		return 1
	}
	cfg := config.Load()
	info, err := review.Parse(parsed.URL)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		_ = json.NewEncoder(stdout).Encode(map[string]string{"error": err.Error()})
		return 1
	}
	plat, err := cfg.PlatformFor(info, store)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		_ = json.NewEncoder(stdout).Encode(map[string]string{"error": err.Error()})
		return 1
	}
	prov, err := cfg.NewProvider(parsed.Provider, parsed.Model, store)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		_ = json.NewEncoder(stdout).Encode(map[string]string{"error": err.Error()})
		return 1
	}
	focus := parsed.Focus
	if len(focus) == 0 {
		focus = cfg.Focus
	}
	maxC := parsed.MaxComments
	if maxC == 0 {
		maxC = cfg.MaxComments
	}
	result, err := review.Run(ctx, review.Options{
		URL:         parsed.URL,
		Provider:    prov,
		Platform:    plat,
		Focus:       focus,
		MaxComments: maxC,
		DryRun:      parsed.DryRun,
	})
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		_ = json.NewEncoder(stdout).Encode(map[string]string{"error": err.Error()})
		return 1
	}
	if result.Comments == nil {
		result.Comments = []review.Comment{}
	}
	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	return 0
}

// Usage is printed for -h / help.
func Usage() string {
	return strings.TrimSpace(`
mr-reviewer — AI merge request reviewer

Usage:
  mr-reviewer                         start the TUI
  mr-reviewer --config                open the settings panel
  mr-reviewer review <url> [flags]    headless review (JSON on stdout)
  mr-reviewer serve [--host 127.0.0.1] [--port 8080]
  mr-reviewer auth login <provider> [--api-key [TOKEN]] [--device]
  mr-reviewer auth status
  mr-reviewer auth logout <provider>

Review flags:
  --dry-run, --no-post   do not post (default)
  --post                 post inline comments + summary
  --provider NAME        anthropic, openai, xai, google, kimi, deepseek, echo
  --model ID             model identifier
  --focus a,b,c          review focus areas
  --max-comments N       non-error comment budget
`) + "\n"
}
