package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

// Anthropic is the Messages API adapter.
type Anthropic struct {
	BaseURL string
	Model   string
	Key     func(context.Context) (string, error)
	HTTP    *http.Client
}

func (p *Anthropic) Name() string { return "anthropic" }

func (p *Anthropic) endpoint() string {
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if base == "" {
		base = "https://api.anthropic.com"
	}
	if strings.HasSuffix(base, "/messages") {
		return base
	}
	if strings.HasSuffix(base, "/v1") {
		return base + "/messages"
	}
	return base + "/v1/messages"
}

func (p *Anthropic) Review(ctx context.Context, system, user string) (review.Result, error) {
	key, err := p.Key(ctx)
	if err != nil {
		return review.Result{}, err
	}
	reqBody := map[string]any{
		"model":      p.Model,
		"max_tokens": 4096,
		"system":     system,
		"messages":   []map[string]string{{"role": "user", "content": user}},
		"tools": []map[string]any{{
			"name":         ReviewToolName,
			"description":  "Submit a structured code review",
			"input_schema": json.RawMessage(reviewToolJSON),
		}},
		"tool_choice": map[string]string{"type": "tool", "name": ReviewToolName},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return review.Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint(), bytes.NewReader(raw))
	if err != nil {
		return review.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")
	cli := p.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 5 * time.Minute}
	}
	resp, err := cli.Do(req)
	if err != nil {
		return review.Result{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return review.Result{}, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return review.Result{}, fmt.Errorf("anthropic API key is invalid or expired")
	}
	if resp.StatusCode != http.StatusOK {
		return review.Result{}, fmt.Errorf("anthropic API %s: %.300s", resp.Status, body)
	}
	var parsed struct {
		Content []struct {
			Type  string          `json:"type"`
			Input json.RawMessage `json:"input"`
			Text  string          `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return review.Result{}, err
	}
	for _, block := range parsed.Content {
		if block.Type == "tool_use" && len(block.Input) > 0 {
			return decodeResult(block.Input)
		}
	}
	for _, block := range parsed.Content {
		if block.Text != "" {
			return parseReviewJSON(block.Text)
		}
	}
	return review.Result{Summary: "Error: Claude did not return a structured review.", Comments: []review.Comment{}}, nil
}
