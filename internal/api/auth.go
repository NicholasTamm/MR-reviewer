package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/jonathanung/mr-reviewer/internal/auth"
)

var errUnsupportedPlatform = errors.New("unsupported platform")

type authSession struct {
	ID                      string
	Kind                    string
	Name                    string
	Method                  string
	Status                  string
	UserCode                string
	VerificationURI         string
	VerificationURIComplete string
	AuthorizationURL        string
	Error                   string
	cancel                  context.CancelFunc
	pending                 *auth.PendingLogin
}

type authSessionStore struct {
	mu   sync.Mutex
	byID map[string]*authSession
}

func newAuthSessionStore() *authSessionStore {
	return &authSessionStore{byID: map[string]*authSession{}}
}

func (s *authSessionStore) put(sess *authSession) {
	s.mu.Lock()
	s.byID[sess.ID] = sess
	s.mu.Unlock()
}

func (s *authSessionStore) get(id string) *authSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byID[id]
}

func (s *authSessionStore) snapshot(id string) (authSessionJSON, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.byID[id]
	if sess == nil {
		return authSessionJSON{}, false
	}
	return sess.json(), true
}

func decodeJSON(r *http.Request, dest any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dest); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("unexpected trailing JSON")
	}
	return nil
}

func (s *Server) handleAuthStart(w http.ResponseWriter, r *http.Request) {
	var req authStartRequest
	if err := decodeJSON(r, &req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	name := strings.ToLower(strings.TrimSpace(req.Name))
	method := strings.ToLower(strings.TrimSpace(req.Method))
	if kind == "provider" {
		name = strings.ToLower(strings.TrimSpace(req.Name))
	}
	if kind != "provider" && kind != "platform" {
		writeDetail(w, http.StatusUnprocessableEntity, "kind must be provider or platform")
		return
	}
	if method != "oauth" && method != "device" {
		writeDetail(w, http.StatusUnprocessableEntity, "method must be oauth or device")
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	sess := &authSession{
		ID: s.id(), Kind: kind, Name: name, Method: method, Status: "pending", cancel: cancel,
	}
	s.sessions.put(sess)
	if method == "device" {
		s.startDeviceSession(ctx, sess)
	} else {
		s.startOAuthSession(ctx, sess)
	}
	out, _ := s.sessions.snapshot(sess.ID)
	writeJSON(w, http.StatusCreated, out)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	out, ok := s.sessions.snapshot(r.PathValue("session_id"))
	if !ok {
		writeDetail(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAuthCancel(w http.ResponseWriter, r *http.Request) {
	sess := s.sessions.get(r.PathValue("session_id"))
	if sess == nil {
		writeDetail(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.cancel != nil {
		sess.cancel()
	}
	s.updateSession(sess.ID, func(cur *authSession) {
		if cur.Status == "pending" {
			cur.Status = "canceled"
		}
	})
	out, _ := s.sessions.snapshot(sess.ID)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAuthPaste(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Callback string `json:"callback"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, "invalid request body")
		return
	}
	sess := s.sessions.get(r.PathValue("session_id"))
	if sess == nil || sess.pending == nil {
		writeDetail(w, http.StatusNotFound, "session not found")
		return
	}
	if err := sess.pending.CompleteWithPaste(req.Callback); err != nil {
		writeDetail(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess.json())
}

func (s *Server) startDeviceSession(ctx context.Context, sess *authSession) {
	cfg, err := s.deviceConfig(sess.Name)
	if err != nil {
		s.failSession(sess.ID, err)
		return
	}
	code, err := cfg.RequestCode(ctx)
	if err != nil {
		s.failSession(sess.ID, err)
		return
	}
	s.updateSession(sess.ID, func(cur *authSession) {
		cur.UserCode = code.UserCode
		cur.VerificationURI = code.VerificationURI
		cur.VerificationURIComplete = code.VerificationURIComplete
	})
	go func() {
		tokens, err := cfg.Poll(ctx, code)
		if err != nil {
			s.failSession(sess.ID, err)
			return
		}
		if err := s.persistTokens(ctx, sess, cfg.ClientID, tokens); err != nil {
			s.failSession(sess.ID, err)
			return
		}
		s.updateSession(sess.ID, func(cur *authSession) { cur.Status = "complete" })
	}()
}

func (s *Server) startOAuthSession(ctx context.Context, sess *authSession) {
	pending, err := s.beginOAuth(sess.Name)
	if err != nil {
		s.failSession(sess.ID, err)
		return
	}
	s.updateSession(sess.ID, func(cur *authSession) {
		cur.pending = pending
		cur.AuthorizationURL = pending.URL
	})
	go func() {
		tokens, err := pending.Wait(ctx)
		if err != nil {
			s.failSession(sess.ID, err)
			return
		}
		if err := s.persistTokens(ctx, sess, pending.ClientID(), tokens); err != nil {
			s.failSession(sess.ID, err)
			return
		}
		s.updateSession(sess.ID, func(cur *authSession) { cur.Status = "complete" })
	}()
}

func (s *Server) persistTokens(ctx context.Context, sess *authSession, clientID string, tokens *auth.Tokens) error {
	if s.Store == nil {
		return errors.New("shared credential store is unavailable")
	}
	if sess.Kind == "platform" {
		target, err := s.platformTarget(sess.Name)
		if err != nil {
			return err
		}
		_, err = auth.CompletePlatformLogin(ctx, s.Store, target, clientID, tokens)
		return err
	}
	_, err := auth.CompleteLogin(ctx, s.Store, sess.Name, tokens)
	return err
}

func (s *Server) deviceConfig(name string) (auth.DeviceConfig, error) {
	if s.DeviceFlow != nil {
		return s.DeviceFlow(name)
	}
	switch name {
	case "github":
		return auth.GitHubDeviceFlow(auth.GitHubOAuthClientID())
	case "gitlab":
		return auth.GitLabDeviceFlow(auth.GitLabOAuthClientID())
	case "xai":
		return auth.XAIDeviceFlow(), nil
	default:
		return auth.DeviceConfig{}, fmt.Errorf("device authorization is not supported for %s", name)
	}
}

func (s *Server) beginOAuth(name string) (*auth.PendingLogin, error) {
	if s.BeginOAuth != nil {
		return s.BeginOAuth(name)
	}
	switch name {
	case "openai":
		return auth.OpenAIFlow().Begin()
	case "xai":
		return auth.XAIFlow().Begin()
	case "gitlab":
		flow, err := auth.GitLabFlow(auth.GitLabOAuthClientID())
		if err != nil {
			return nil, err
		}
		return flow.Begin()
	default:
		return nil, fmt.Errorf("browser OAuth is not supported for %s", name)
	}
}

func (s *Server) failSession(id string, err error) {
	s.updateSession(id, func(cur *authSession) {
		if cur.Status != "canceled" {
			cur.Status = "failed"
			cur.Error = err.Error()
		}
	})
}

func (s *Server) updateSession(id string, fn func(*authSession)) {
	s.sessions.mu.Lock()
	defer s.sessions.mu.Unlock()
	if cur := s.sessions.byID[id]; cur != nil {
		fn(cur)
	}
}

func (sess *authSession) json() authSessionJSON {
	return authSessionJSON{
		SessionID:               sess.ID,
		Kind:                    sess.Kind,
		Name:                    sess.Name,
		Method:                  sess.Method,
		Status:                  sess.Status,
		UserCode:                sess.UserCode,
		VerificationURI:         sess.VerificationURI,
		VerificationURIComplete: sess.VerificationURIComplete,
		AuthorizationURL:        sess.AuthorizationURL,
		Error:                   sess.Error,
	}
}
