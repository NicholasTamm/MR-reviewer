package review

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

const defaultParallelAgents = 2

// ParallelReview partitions files round-robin across agents, reviews concurrently,
// and merges results. Partial agent failure is ignored; all-fail returns an error.
func ParallelReview(ctx context.Context, provider Provider, files []DiffFile, contents map[string]string, focus []string, meta Metadata, agents, maxComments int) (Result, error) {
	if provider == nil {
		return Result{}, fmt.Errorf("review provider is required")
	}
	if agents <= 0 {
		agents = defaultParallelAgents
	}
	if maxComments <= 0 {
		maxComments = 10
	}
	if len(focus) == 0 {
		focus = DefaultFocus
	}

	partitions := make([][]DiffFile, agents)
	for i, df := range files {
		partitions[i%agents] = append(partitions[i%agents], df)
	}

	system := SystemPrompt(focus, maxComments)
	type outcome struct {
		result Result
		err    error
	}
	var (
		mu      sync.Mutex
		results []Result
		wg      sync.WaitGroup
	)
	for _, part := range partitions {
		if len(part) == 0 {
			continue
		}
		part := part
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
			}
			out, err := reviewPartition(ctx, provider, system, part, contents, meta)
			if err != nil {
				return
			}
			mu.Lock()
			results = append(results, out)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	if len(results) == 0 {
		return Result{}, fmt.Errorf("all parallel review agents failed")
	}
	return MergeResults(results), nil
}

func reviewPartition(ctx context.Context, provider Provider, system string, files []DiffFile, contents map[string]string, meta Metadata) (Result, error) {
	paths := map[string]struct{}{}
	for _, df := range files {
		if df.NewPath != "" {
			paths[df.NewPath] = struct{}{}
		}
		if df.OldPath != "" {
			paths[df.OldPath] = struct{}{}
		}
	}
	partial := map[string]string{}
	for path, content := range contents {
		if _, ok := paths[path]; ok {
			partial[path] = content
		}
	}
	user := UserMessage(meta.Title, meta.Description, AnnotateDiff(BuildUnifiedDiff(files)), partial)
	return provider.Review(ctx, system, user)
}

// MergeResults concatenates summaries, keeps the first comment per (file, line), and sorts.
func MergeResults(results []Result) Result {
	if len(results) == 0 {
		return Result{Summary: "", Comments: []Comment{}}
	}
	var summaries []string
	seen := map[string]struct{}{}
	var comments []Comment
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, r.Summary)
		}
		for _, c := range r.Comments {
			key := fmt.Sprintf("%s\x00%d", c.File, c.Line)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			comments = append(comments, c)
		}
	}
	sort.SliceStable(comments, func(i, j int) bool {
		if comments[i].File != comments[j].File {
			return comments[i].File < comments[j].File
		}
		return comments[i].Line < comments[j].Line
	})
	if comments == nil {
		comments = []Comment{}
	}
	summary := ""
	for i, s := range summaries {
		if i > 0 {
			summary += "\n\n---\n\n"
		}
		summary += s
	}
	return Result{Summary: summary, Comments: comments}
}
