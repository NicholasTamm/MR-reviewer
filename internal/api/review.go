package api

import (
	"context"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

func (s *Server) runReview(jobID string, req reviewRequest) {
	ctx := context.Background()
	s.Jobs.update(jobID, func(j *job) {
		j.Status = "fetching"
		j.Progress = "Parsing URL..."
	})

	plat, err := s.platformForURL(req.URL)
	if err != nil {
		s.fail(jobID, err)
		return
	}
	prov, err := s.NewProvider(req.Provider, req.Model)
	if err != nil {
		s.fail(jobID, err)
		return
	}

	info, err := review.Parse(req.URL)
	if err != nil {
		s.fail(jobID, err)
		return
	}
	s.Jobs.update(jobID, func(j *job) {
		j.Platform = plat
		j.Info = info
		j.Progress = "Fetching MR changes..."
	})

	out, err := review.Execute(ctx, review.Options{
		URL:               req.URL,
		Provider:          prov,
		Platform:          plat,
		Focus:             req.Focus,
		MaxComments:       req.MaxComments,
		DryRun:            true,
		Parallel:          req.Parallel,
		ParallelThreshold: s.Settings.ParallelThreshold,
		Progress: func(status, message string) {
			s.Jobs.update(jobID, func(j *job) {
				if status != "" {
					j.Status = status
				}
				j.Progress = message
			})
		},
	})
	if err != nil {
		s.fail(jobID, err)
		return
	}

	comments := make([]commentDetail, 0, len(out.Result.Comments))
	for _, c := range out.Result.Comments {
		comments = append(comments, commentDetail{
			ID:          s.id(),
			File:        c.File,
			Line:        c.Line,
			Body:        c.Body,
			Severity:    c.Severity,
			IsNewLine:   c.IsNewLine,
			DiffContext: review.ExtractDiffContext(c.File, c.Line, out.DiffLines, 3),
			Approved:    true,
		})
	}
	meta := metadataResponse{
		Title:        out.Result.Meta.Title,
		Description:  out.Result.Meta.Description,
		SourceBranch: out.Result.Meta.SourceBranch,
		TargetBranch: out.Result.Meta.TargetBranch,
		WebURL:       out.Result.Meta.WebURL,
	}

	if req.AutoPost {
		s.Jobs.update(jobID, func(j *job) { j.Progress = "Posting review..." })
		if err := plat.PostReview(ctx, out.Info, out.Result, out.DiffLines); err != nil {
			s.fail(jobID, err)
			return
		}
		s.Jobs.update(jobID, func(j *job) {
			j.Status = "posted"
			j.Progress = "Posted"
			j.Summary = out.Result.Summary
			j.Comments = comments
			j.Metadata = meta
			j.Info = out.Info
			j.Platform = plat
			j.DiffLines = out.DiffLines
		})
		return
	}

	s.Jobs.update(jobID, func(j *job) {
		j.Status = "complete"
		j.Progress = "Review complete"
		j.Summary = out.Result.Summary
		j.Comments = comments
		j.Metadata = meta
		j.Info = out.Info
		j.Platform = plat
		j.DiffLines = out.DiffLines
	})
}

func (s *Server) platformForURL(raw string) (review.Platform, error) {
	info, err := review.Parse(raw)
	if err != nil {
		return nil, err
	}
	return s.NewPlatform(info)
}

func (s *Server) fail(jobID string, err error) {
	msg := err.Error()
	s.Jobs.update(jobID, func(j *job) {
		j.Status = "failed"
		j.Progress = ""
		j.Error = msg
		j.ErrorType = classifyError(err)
	})
}
