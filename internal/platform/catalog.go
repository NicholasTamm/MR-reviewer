package platform

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

const (
	catalogPageSize = 100
	maxCatalogPages = 20
)

// Catalog discovers projects and their open reviews without fetching a global
// review list. It is independent of the fetch-and-post review client contract.
type Catalog interface {
	ListProjects(ctx context.Context, search string) ([]review.Project, error)
	ListProjectReviews(ctx context.Context, project review.Project, search string) ([]review.ReviewSummary, error)
}

func catalogError(platform string, resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	message := strings.TrimSpace(string(body))
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s authentication failed: sign in again or provide a token with repository access", platform)
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("%s API rate limit exceeded; retry after %s", platform, resp.Header.Get("X-RateLimit-Reset"))
		}
		return fmt.Errorf("%s authorization failed: the token cannot access this resource", platform)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s API rate limit exceeded; retry after %s", platform, resp.Header.Get("Retry-After"))
	default:
		return fmt.Errorf("%s API request failed: %s %s", platform, resp.Status, message)
	}
}

func matchesCatalogSearch(search string, values ...string) bool {
	search = strings.ToLower(strings.TrimSpace(search))
	if search == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}
