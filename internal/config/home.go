package config

import (
	"os"
	"path/filepath"
)

// HomeDir is ~/.mr-reviewer, or MR_REVIEWER_HOME when set.
func HomeDir() string {
	if p := os.Getenv("MR_REVIEWER_HOME"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "mr-reviewer")
	}
	root := filepath.Join(home, ".mr-reviewer")
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	return root
}

// ConfigPath is the persisted settings file (MR_REVIEWER_CONFIG overrides).
func ConfigPath() string {
	if p := os.Getenv("MR_REVIEWER_CONFIG"); p != "" {
		return p
	}
	return filepath.Join(HomeDir(), "config.json")
}

// ProvidersPath prefers providers.jsonc then providers.json under HomeDir.
func ProvidersPath() string {
	if p := os.Getenv("MR_REVIEWER_PROVIDERS"); p != "" {
		return p
	}
	dir := HomeDir()
	jsonc := filepath.Join(dir, "providers.jsonc")
	if _, err := os.Stat(jsonc); err == nil {
		return jsonc
	}
	json := filepath.Join(dir, "providers.json")
	if _, err := os.Stat(json); err == nil {
		return json
	}
	return jsonc
}
