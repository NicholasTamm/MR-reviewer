package provider

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

var fencedJSON = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func parseReviewJSON(raw string) (review.Result, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return review.Result{Summary: "Empty model response.", Comments: []review.Comment{}}, nil
	}
	if r, err := decodeResult([]byte(raw)); err == nil {
		return r, nil
	}
	if m := fencedJSON.FindStringSubmatch(raw); len(m) == 2 {
		if r, err := decodeResult([]byte(m[1])); err == nil {
			return r, nil
		}
	}
	// Last-ditch: first { ... } object.
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			if r, err := decodeResult([]byte(raw[i : j+1])); err == nil {
				return r, nil
			}
		}
	}
	return review.Result{Summary: raw, Comments: []review.Comment{}}, nil
}

func decodeResult(b []byte) (review.Result, error) {
	var r review.Result
	if err := json.Unmarshal(b, &r); err != nil {
		return review.Result{}, err
	}
	if r.Comments == nil {
		r.Comments = []review.Comment{}
	}
	for i := range r.Comments {
		if r.Comments[i].Severity == "" {
			r.Comments[i].Severity = "info"
		}
	}
	return r, nil
}
