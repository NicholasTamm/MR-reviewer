package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Models is one provider's discovered review models. Error never contains secrets.
type Models struct {
	Provider  string   `json:"provider"`
	Models    []string `json:"models"`
	Available bool     `json:"available"`
	Error     string   `json:"error,omitempty"`
}

// Unavailable is a secret-free catalog miss.
func Unavailable(provider, err string) Models {
	return Models{Provider: provider, Models: []string{}, Available: false, Error: err}
}

// FilterReviewModels drops embeddings, TTS, and other non-review IDs.
func FilterReviewModels(name string, ids []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] || !reviewModel(name, id) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func reviewModel(_ string, id string) bool {
	lower := strings.ToLower(id)
	skip := []string{"embed", "tts", "whisper", "dall-e", "dalle", "moderation", "babbage", "ada-", "davinci", "audio", "realtime", "transcribe", "image", "sora"}
	for _, part := range skip {
		if strings.Contains(lower, part) {
			return false
		}
	}
	return true
}

// DiscoverQuery is one provider lookup for TUI and the HTTP catalog.
type DiscoverQuery struct {
	Name         string
	BaseURL      string
	Key          string
	CustomModels []string
}

// DiscoverOne returns an explicit custom list or models confirmed by the provider's live API.
func DiscoverOne(ctx context.Context, client *http.Client, q DiscoverQuery) Models {
	name := Canonical(strings.ToLower(strings.TrimSpace(q.Name)))
	if name == "echo" {
		return Models{Provider: name, Models: []string{"echo"}, Available: true}
	}
	if len(q.CustomModels) > 0 {
		models := append([]string{}, q.CustomModels...)
		sort.Strings(models)
		return Models{Provider: name, Models: models, Available: true}
	}
	if q.Key == "" && name != "ollama" {
		return Unavailable(name, "Provider credentials are not configured.")
	}
	live, err := ListRemoteModels(ctx, client, name, q.BaseURL, q.Key)
	if err != nil {
		return Unavailable(name, "Unable to retrieve available models.")
	}
	filtered := FilterReviewModels(name, live)
	if len(filtered) == 0 {
		return Unavailable(name, "No review models are available.")
	}
	sort.Strings(filtered)
	return Models{Provider: name, Models: filtered, Available: true}
}

// ListRemoteModels fetches model IDs from a provider endpoint. The key is used
// only as an HTTP credential and is never returned.
func ListRemoteModels(ctx context.Context, client *http.Client, name, base, key string) ([]string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	name = Canonical(strings.ToLower(strings.TrimSpace(name)))
	base = strings.TrimRight(base, "/")
	switch name {
	case "anthropic":
		return listAnthropicModels(ctx, client, base, key)
	case "google":
		return listGoogleModels(ctx, client, base, key)
	case "ollama":
		return listOllamaModels(ctx, client, base)
	default:
		return listOpenAIModels(ctx, client, base, key)
	}
}

func listAnthropicModels(ctx context.Context, client *http.Client, base, key string) ([]string, error) {
	if base == "" {
		base = BaseURL("anthropic")
	}
	endpoint := base
	if !strings.HasSuffix(endpoint, "/models") {
		if strings.HasSuffix(endpoint, "/v1") {
			endpoint += "/models"
		} else {
			endpoint += "/v1/models"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	return decodeIDList(client, req, "data")
}

func listOpenAIModels(ctx context.Context, client *http.Client, base, key string) ([]string, error) {
	if base == "" {
		return nil, fmt.Errorf("missing base URL")
	}
	endpoint := base
	if !strings.HasSuffix(endpoint, "/models") {
		endpoint += "/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return decodeIDList(client, req, "data")
}

func listGoogleModels(ctx context.Context, client *http.Client, base, key string) ([]string, error) {
	if base == "" {
		base = BaseURL("google")
	}
	endpoint := strings.TrimRight(base, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", key)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google models %s", resp.Status)
	}
	var parsed struct {
		Models []struct {
			Name                string   `json:"name"`
			SupportedGeneration []string `json:"supportedGenerationMethods"`
			SupportedActions    []string `json:"supportedActions"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range parsed.Models {
		actions := append(append([]string{}, m.SupportedActions...), m.SupportedGeneration...)
		if len(actions) > 0 && !hasGenerateContent(actions) {
			continue
		}
		if m.Name != "" {
			out = append(out, m.Name)
		}
	}
	return out, nil
}

func listOllamaModels(ctx context.Context, client *http.Client, base string) ([]string, error) {
	host := strings.TrimRight(base, "/")
	host = strings.TrimSuffix(host, "/v1")
	if host == "" {
		host = "http://localhost:11434"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama tags %s", resp.Status)
	}
	var parsed struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range parsed.Models {
		name := m.Model
		if name == "" {
			name = m.Name
		}
		if name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

func decodeIDList(client *http.Client, req *http.Request, key string) ([]string, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models %s", resp.Status)
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	raw, ok := parsed[key]
	if !ok {
		return nil, fmt.Errorf("models response missing %s", key)
	}
	var items []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	var out []string
	for _, item := range items {
		if item.ID != "" {
			out = append(out, item.ID)
		}
	}
	return out, nil
}

func hasGenerateContent(actions []string) bool {
	for _, a := range actions {
		if strings.EqualFold(a, "generateContent") {
			return true
		}
	}
	return false
}
