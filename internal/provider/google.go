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

// Google is the Gemini generateContent adapter.
type Google struct {
	BaseURL string
	Model   string
	Key     func(context.Context) (string, error)
	HTTP    *http.Client
}

func (p *Google) Name() string { return "google" }

func (p *Google) Review(ctx context.Context, system, user string) (review.Result, error) {
	key, err := p.Key(ctx)
	if err != nil {
		return review.Result{}, err
	}
	model := p.Model
	model = strings.TrimPrefix(model, "models/")
	base := strings.TrimRight(p.BaseURL, "/")
	if base == "" {
		base = "https://generativelanguage.googleapis.com/v1beta"
	}
	endpoint := fmt.Sprintf("%s/models/%s:generateContent", base, model)
	reqBody := map[string]any{
		"systemInstruction": map[string]any{
			"parts": []map[string]string{{"text": system}},
		},
		"contents": []map[string]any{{
			"role":  "user",
			"parts": []map[string]string{{"text": user}},
		}},
		"generationConfig": map[string]any{
			"responseMimeType": "application/json",
			"responseSchema":   json.RawMessage(reviewToolJSON),
		},
	}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return review.Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return review.Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", key)
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
		return review.Result{}, fmt.Errorf("google API %s: %.300s", resp.Status, body)
	}
	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return review.Result{}, err
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return review.Result{}, fmt.Errorf("google: response has no candidates")
	}
	return parseReviewJSON(parsed.Candidates[0].Content.Parts[0].Text)
}
