package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/cli"
	"github.com/jonathanung/mr-reviewer/internal/tui"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}
	switch args[0] {
	case "review":
		return cli.RunReview(context.Background(), args[1:], os.Stdout, os.Stderr)
	case "auth":
		return cli.RunAuth(args[1:], os.Stdout, os.Stderr)
	case "help", "-h", "--help":
		fmt.Fprint(os.Stdout, cli.Usage())
		return 0
	default:
		if strings.HasPrefix(args[0], "http://") || strings.HasPrefix(args[0], "https://") {
			return cli.RunReview(context.Background(), args, os.Stdout, os.Stderr)
		}
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", args[0], cli.Usage())
		return 1
	}
}
