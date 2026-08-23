package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonathanung/mr-reviewer/internal/auth"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func startServe(t *testing.T, args []string) (port int, cancel context.CancelFunc, done <-chan int, stderr *bytes.Buffer) {
	t.Helper()
	port = freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	var out, errb bytes.Buffer
	exit := make(chan int, 1)
	all := append([]string{"--port", strconv.Itoa(port)}, args...)
	go func() {
		exit <- RunServe(ctx, all, &out, &errb)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/api/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				return port, cancel, exit, &errb
			}
		}
		select {
		case code := <-exit:
			t.Fatalf("serve exited %d: %s", code, errb.String())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	t.Fatalf("serve did not become healthy: %s", errb.String())
	return 0, cancel, exit, &errb
}

func serveJSON(t *testing.T, method, url, token, body string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestParseServeArgsLoopbackOnly(t *testing.T) {
	got, err := ParseServeArgs(nil)
	if err != nil || got.Host != "127.0.0.1" || got.Port != 8080 {
		t.Fatalf("%+v %v", got, err)
	}
	got, err = ParseServeArgs([]string{"--port", "9999"})
	if err != nil || got.Port != 9999 || got.Host != "127.0.0.1" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := ParseServeArgs([]string{"--host", "0.0.0.0"}); err == nil || !strings.Contains(err.Error(), "127.0.0.1") {
		t.Fatalf("err = %v", err)
	}
	if _, err := ParseServeArgs([]string{"--host", "localhost"}); err == nil {
		t.Fatal("localhost must be rejected")
	}
}

func TestRunServeRequiresToken(t *testing.T) {
	t.Setenv("MR_REVIEWER_TOKEN", "")
	t.Setenv("MR_REVIEWER_HOME", t.TempDir())
	var out, errb bytes.Buffer
	if code := RunServe(context.Background(), []string{"--port", strconv.Itoa(freePort(t))}, &out, &errb); code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(errb.String(), "MR_REVIEWER_TOKEN") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunServeHealthAuthAndHomeStore(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MR_REVIEWER_HOME", dir)
	t.Setenv("MR_REVIEWER_CONFIG", "")
	t.Setenv("MR_REVIEWER_PROVIDERS", "")
	t.Setenv("MR_REVIEWER_AUTH", filepath.Join(dir, "auth.json"))
	t.Setenv("MR_REVIEWER_TOKEN", "serve-token")
	t.Setenv("MR_REVIEWER_PROVIDER", "echo")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")

	port, cancel, done, errb := startServe(t, nil)
	defer cancel()
	base := "http://127.0.0.1:" + strconv.Itoa(port)

	health := serveJSON(t, http.MethodGet, base+"/api/health", "", "")
	if health.StatusCode != 200 || decodeBody(t, health)["status"] != "ok" {
		t.Fatal("health")
	}
	if serveJSON(t, http.MethodGet, base+"/api/config/defaults", "", "").StatusCode != 403 {
		t.Fatal("missing token")
	}
	if serveJSON(t, http.MethodGet, base+"/api/config/defaults", "wrong", "").StatusCode != 403 {
		t.Fatal("wrong token")
	}
	resp := serveJSON(t, http.MethodGet, base+"/api/config/defaults", "serve-token", "")
	if resp.StatusCode != 200 {
		t.Fatal(resp.Status)
	}
	if decodeBody(t, resp)["provider"] != "echo" {
		t.Fatal("defaults")
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d %s", code, errb.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not stop")
	}
}

func TestRunServeSubmitPollPost(t *testing.T) {
	var posted bool
	gh := githubFixture(t, &posted)
	t.Cleanup(gh.Close)

	dir := t.TempDir()
	authPath := filepath.Join(dir, "auth.json")
	t.Setenv("MR_REVIEWER_HOME", dir)
	t.Setenv("MR_REVIEWER_CONFIG", "")
	t.Setenv("MR_REVIEWER_PROVIDERS", "")
	t.Setenv("MR_REVIEWER_AUTH", authPath)
	t.Setenv("MR_REVIEWER_TOKEN", "serve-token")
	t.Setenv("MR_REVIEWER_PROVIDER", "echo")
	t.Setenv("MR_REVIEWER_GITHUB_API", gh.URL)
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	st, err := auth.OpenStore(authPath)
	if err != nil {
		t.Fatal(err)
	}
	target, err := auth.NewPlatformTarget("github", "https://github.com", gh.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetPlatform(context.Background(), target, auth.PlatformCredential{Type: auth.PlatformPAT, Token: "test-token"}); err != nil {
		t.Fatal(err)
	}

	port, cancel, done, errb := startServe(t, nil)
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}()
	base := "http://127.0.0.1:" + strconv.Itoa(port)
	token := "serve-token"

	resp := serveJSON(t, http.MethodPost, base+"/api/reviews", token, `{"url":"https://github.com/owner/repo/pull/1","provider":"echo","model":"echo"}`)
	if resp.StatusCode != 201 {
		t.Fatalf("submit %d %v", resp.StatusCode, decodeBody(t, resp))
	}
	jobID, _ := decodeBody(t, resp)["job_id"].(string)
	if jobID == "" {
		t.Fatal("job_id")
	}

	var status map[string]any
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		stResp := serveJSON(t, http.MethodGet, base+"/api/reviews/"+jobID, token, "")
		status = decodeBody(t, stResp)
		s, _ := status["status"].(string)
		if s == "complete" || s == "posted" || s == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if status["status"] != "complete" {
		t.Fatalf("status=%v stderr=%s", status, errb.String())
	}

	results := decodeBody(t, serveJSON(t, http.MethodGet, base+"/api/reviews/"+jobID+"/results", token, ""))
	comments, _ := results["comments"].([]any)
	if results["summary"] == "" || len(comments) == 0 {
		t.Fatalf("results=%v", results)
	}
	first, _ := comments[0].(map[string]any)
	commentID, _ := first["id"].(string)
	body, _ := json.Marshal(map[string]any{
		"comment_ids": []string{commentID},
		"summary":     results["summary"],
	})
	post := serveJSON(t, http.MethodPost, base+"/api/reviews/"+jobID+"/post", token, string(body))
	if post.StatusCode != 200 {
		t.Fatalf("post %d %v", post.StatusCode, decodeBody(t, post))
	}
	if !posted {
		t.Fatal("platform post not called")
	}
	if decodeBody(t, post)["status"] != "posted" {
		t.Fatal("posted status")
	}
}
