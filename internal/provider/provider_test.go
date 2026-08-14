package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jonathanung/mr-reviewer/internal/auth"
)

func TestEchoReviewsFirstAddedLine(t *testing.T) {
	user := "## Diff\n```diff\n+++ b/hello.py\n+[L2]import sys\n```\n"
	got, err := Echo{}.Review(context.Background(), "sys", user)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary == "" || len(got.Comments) != 1 {
		t.Fatalf("%+v", got)
	}
	if got.Comments[0].File != "hello.py" || got.Comments[0].Line != 2 || got.Comments[0].Severity == "" {
		t.Fatalf("%+v", got.Comments[0])
	}
}

func TestAnthropicToolUseViaHTTPTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "ak" {
			t.Errorf("key = %q", r.Header.Get("x-api-key"))
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "submit_review") {
			t.Errorf("body missing tool: %s", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{
				"type": "tool_use",
				"name": "submit_review",
				"input": map[string]any{
					"summary": "Looks solid.",
					"comments": []map[string]any{{
						"file": "a.go", "line": 3, "body": "*error* **BUG:** x", "severity": "error",
					}},
				},
			}},
		})
	}))
	defer srv.Close()

	p := &Anthropic{
		BaseURL: srv.URL,
		Model:   "claude-test",
		Key:     func(context.Context) (string, error) { return "ak", nil },
	}
	got, err := p.Review(context.Background(), "system", "user")
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "Looks solid." || len(got.Comments) != 1 || got.Comments[0].File != "a.go" || got.Comments[0].Line != 3 {
		t.Fatalf("%+v", got)
	}
}

func TestOpenAICompatToolCallViaHTTPTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		args, _ := json.Marshal(map[string]any{
			"summary":  "ok",
			"comments": []map[string]any{{"file": "b.py", "line": 4, "body": "n", "severity": "warning"}},
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"tool_calls": []map[string]any{{
						"function": map[string]any{"name": "submit_review", "arguments": string(args)},
					}},
				},
			}},
		})
	}))
	defer srv.Close()
	p := &OpenAICompat{
		NameID:  "openai",
		BaseURL: srv.URL,
		Model:   "gpt-test",
		Key:     func(context.Context) (string, error) { return "sk", nil },
	}
	got, err := p.Review(context.Background(), "s", "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "ok" || got.Comments[0].File != "b.py" {
		t.Fatalf("%+v", got)
	}
}

func TestGoogleGenerateContentViaHTTPTest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "gk" {
			t.Errorf("key = %q", r.Header.Get("x-goog-api-key"))
		}
		if !strings.Contains(r.URL.Path, ":generateContent") {
			t.Errorf("path = %s", r.URL.Path)
		}
		payload, _ := json.Marshal(map[string]any{
			"summary":  "g",
			"comments": []map[string]any{{"file": "c.ts", "line": 1, "body": "n", "severity": "info"}},
		})
		_ = json.NewEncoder(w).Encode(map[string]any{
			"candidates": []map[string]any{{
				"content": map[string]any{
					"parts": []map[string]any{{"text": string(payload)}},
				},
			}},
		})
	}))
	defer srv.Close()
	p := &Google{
		BaseURL: srv.URL,
		Model:   "gemini-test",
		Key:     func(context.Context) (string, error) { return "gk", nil },
	}
	got, err := p.Review(context.Background(), "s", "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != "g" || got.Comments[0].File != "c.ts" {
		t.Fatalf("%+v", got)
	}
}

func TestFactoryGeminiAliasAndEcho(t *testing.T) {
	st, err := auth.OpenStore(filepath.Join(t.TempDir(), "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := New("echo", "", st, Options{})
	if err != nil || p.Name() != "echo" {
		t.Fatalf("%v %v", p, err)
	}
	p, err = New("gemini", "gemini-2.5-flash", st, Options{GoogleURL: "http://example.invalid"})
	if err != nil || p.Name() != "google" {
		t.Fatalf("%v %v", p, err)
	}
}
