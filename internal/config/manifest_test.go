package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/config"
)

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")
	content := `{"repos":[{"name":"payments-api","owner":"acme","provider":"github","default_branch":"main"}]}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := config.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(m.Repos) != 1 || m.Repos[0].Name != "payments-api" {
		t.Errorf("unexpected manifest: %+v", m)
	}
}

func TestLoadManifest_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{"repos":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := config.LoadManifest(path)
	if err == nil {
		t.Fatal("expected error for empty manifest")
	}
}
