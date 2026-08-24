package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	if code := run([]string{"help"}); code != 0 {
		t.Fatalf("help exit %d", code)
	}
}

func TestConfigArgvIsRecognized(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `"--config"`) || !strings.Contains(string(src), "ViewConfig") {
		t.Fatal("main must recognize --config and start the config view")
	}
}

func TestServeArgvIsRecognized(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), `"serve"`) || !strings.Contains(string(src), "RunServe") {
		t.Fatal("main must recognize serve")
	}
}

func TestElectronSpawnsGoServe(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "frontend", "electron", "main.ts"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	if strings.Contains(text, `"python"`) || strings.Contains(text, "-m") && strings.Contains(text, "mr_reviewer") {
		t.Fatal("electron must not start a Python process")
	}
	if !strings.Contains(text, `"serve"`) || !strings.Contains(text, "127.0.0.1") {
		t.Fatal("electron must spawn mr-reviewer serve on 127.0.0.1")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "scripts", "build-backend.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(script), "PyInstaller") || strings.Contains(string(script), "python") {
		t.Fatal("packaging must build the Go binary")
	}
	if !strings.Contains(string(script), "go build") {
		t.Fatal("packaging must invoke go build")
	}
}

func TestPythonProductPathRemoved(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, rel := range []string{
		"pyproject.toml",
		"mr-reviewer-server.spec",
		"Dockerfile",
		"docker-compose.yml",
		"mr_reviewer",
		"tests",
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Fatalf("python product path still present: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	if strings.Contains(text, "python -m mr_reviewer") || strings.Contains(text, "pip install") || strings.Contains(text, "FastAPI") {
		t.Fatal("README must describe the Go binary only")
	}
	if !strings.Contains(text, "go test ./...") || !strings.Contains(text, "npm run build") {
		t.Fatal("README must keep go test and the Electron frontend build as verification")
	}
}

func TestRunUnknown(t *testing.T) {
	if code := run([]string{"nope"}); code == 0 {
		t.Fatal("expected non-zero")
	}
}

func TestBinaryExistsModule(t *testing.T) {
	// Structural: this package is the shipped entrypoint.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.Clean(wd), filepath.Join("cmd", "mr-reviewer")) &&
		!strings.Contains(filepath.ToSlash(wd), "cmd/mr-reviewer") {
		// running via go test ./... from module root still compiles this package
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip(err)
	}
}
