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

// OpenAICompat talks to OpenAI-compatible /chat/completions (openai, xai, kimi, deepseek, ollama).
type OpenAICompat struct {
	NameID  string
	BaseURL string
	Model   string
	Key     func(context.Context) (string, error)
	HTTP    *http.Client
}

func (p *OpenAICompat) Name() string { return p.NameID }

func (p *OpenAICompat) Review(ctx context.Context, system, user string) (review.Result, error) {
	key, err := p.Key(ctx)
	if err != nil {
		return review.Result{}, err
	}
	reqBody := map[string]any{
		"model": p.Model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name":        ReviewToolName,
				"description": "Submit a structured code review",
				"parameters":  json.RawMessage(reviewToolJSON),
			},
		}},
		"tool_choice": map[string]any{
			"type":     "function",
			"function": map[string]string{"name": ReviewToolName},
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return review.Result{}, err
	}
	endpoint := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return review.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
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
	if resp.StatusCode != http.StatusOK {
		return review.Result{}, fmt.Errorf("%s API %s: %.300s", p.NameID, resp.Status, body)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content   any `json:"content"`
				ToolCalls []struct {
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return review.Result{}, err
	}
	if len(parsed.Choices) == 0 {
		return review.Result{}, fmt.Errorf("%s: response has no choices", p.NameID)
	}
	msg := parsed.Choices[0].Message
	if len(msg.ToolCalls) > 0 && msg.ToolCalls[0].Function.Arguments != "" {
		return decodeResult([]byte(msg.ToolCalls[0].Function.Arguments))
	}
	return parseReviewJSON(contentString(msg.Content))
}

func contentString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
