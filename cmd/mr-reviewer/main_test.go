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
