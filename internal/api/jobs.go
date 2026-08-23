package api

import (
	"sync"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/review"
)

type job struct {
	ID        string
	URL       string
	Status    string
	Progress  string
	CreatedAt time.Time
	Error     string
	ErrorType string
	Summary   string
	Comments  []commentDetail
	Metadata  metadataResponse
	Info      review.Info
	Platform  review.Platform
	DiffLines []review.DiffLine
}

type jobStore struct {
	mu   sync.Mutex
	jobs map[string]*job
}

func newJobStore() *jobStore {
	return &jobStore{jobs: map[string]*job{}}
}

func (s *jobStore) create(j *job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[j.ID] = j
}

func (s *jobStore) get(id string) *job {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return nil
	}
	cp := *j
	cp.Comments = append([]commentDetail(nil), j.Comments...)
	return &cp
}

func (s *jobStore) update(id string, fn func(*job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return
	}
	fn(j)
}

func (s *jobStore) transition(id, from, to string) *job {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil || j.Status != from {
		return nil
	}
	j.Status = to
	cp := *j
	cp.Comments = append([]commentDetail(nil), j.Comments...)
	return &cp
}

func (s *jobStore) editComment(id, commentID, body string) *commentDetail {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return nil
	}
	for i := range j.Comments {
		if j.Comments[i].ID == commentID {
			j.Comments[i].Body = body
			c := j.Comments[i]
			return &c
		}
	}
	return nil
}

func (j *job) status() jobStatus {
	return jobStatus{
		JobID:     j.ID,
		Status:    j.Status,
		Progress:  strPtr(j.Progress),
		Error:     strPtr(j.Error),
		ErrorType: strPtr(j.ErrorType),
		CreatedAt: formatTime(j.CreatedAt),
		URL:       j.URL,
	}
}
