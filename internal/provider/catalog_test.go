package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestListAnthropicModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "secret-key" {
			t.Errorf("key leaked path or missing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"id": "claude-b"}, {"id": "claude-a"}},
		})
	}))
	defer srv.Close()
	got, err := ListRemoteModels(context.Background(), srv.Client(), "anthropic", srv.URL, "secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "claude-b,claude-a" {
		t.Fatalf("%v", got)
	}
}

func TestListGoogleFiltersGenerateContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "models/gemini-review", "supportedActions": []string{"generateContent"}},
				{"name": "models/embedding", "supportedActions": []string{"embedContent"}},
			},
		})
	}))
	defer srv.Close()
	got, err := ListRemoteModels(context.Background(), srv.Client(), "google", srv.URL, "k")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "models/gemini-review" {
		t.Fatalf("%v", got)
	}
}

func TestListOllamaModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]string{{"model": "qwen:latest"}, {"name": "llama:latest"}},
		})
	}))
	defer srv.Close()
	got, err := ListRemoteModels(context.Background(), srv.Client(), "ollama", srv.URL+"/v1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

func TestUnavailableHasNoSecret(t *testing.T) {
	m := Unavailable("anthropic", "ANTHROPIC_API_KEY is not set")
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), "sk-") {
		t.Fatalf("secret in %s", raw)
	}
	if m.Available || m.Error == "" {
		t.Fatalf("%+v", m)
	}
}
