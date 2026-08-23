package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/api"
	"github.com/jonathanung/mr-reviewer/internal/auth"
	"github.com/jonathanung/mr-reviewer/internal/config"
)

type ServeArgs struct {
	Host string
	Port int
}

func ParseServeArgs(args []string) (ServeArgs, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	host := fs.String("host", "127.0.0.1", "listen address (127.0.0.1 only)")
	port := fs.Int("port", 8080, "listen port")
	if err := fs.Parse(args); err != nil {
		return ServeArgs{}, err
	}
	out := ServeArgs{Host: strings.TrimSpace(*host), Port: *port}
	if out.Host != "127.0.0.1" {
		return ServeArgs{}, fmt.Errorf("serve only binds 127.0.0.1")
	}
	if out.Port <= 0 || out.Port > 65535 {
		return ServeArgs{}, fmt.Errorf("invalid port %d", out.Port)
	}
	return out, nil
}

func RunServe(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	parsed, err := ParseServeArgs(args)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	token := strings.TrimSpace(os.Getenv("MR_REVIEWER_TOKEN"))
	if token == "" {
		fmt.Fprintln(stderr, "MR_REVIEWER_TOKEN is required")
		return 1
	}
	store, err := auth.OpenStore(auth.DefaultPath())
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	addr := net.JoinHostPort(parsed.Host, strconv.Itoa(parsed.Port))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           api.New(config.Load(), store, token).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	fmt.Fprintf(stdout, "listening on http://%s\n", addr)

	errc := make(chan error, 1)
	go func() {
		errc <- httpSrv.Serve(ln)
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
		err := <-errc
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		return 0
	case err := <-errc:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		return 0
	}
}
