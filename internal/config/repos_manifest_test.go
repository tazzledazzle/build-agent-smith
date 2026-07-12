package config_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tazzledazzle/build-agent-smith/internal/config"
)

func TestCheckedInManifest_HasFiveRepos(t *testing.T) {
	// make run logs "listening on :8080 (5 repos)" from configs/repos.json
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "configs", "repos.json")
	m, err := config.LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest(%s): %v", path, err)
	}
	if len(m.Repos) != 5 {
		t.Fatalf("repos = %d, want 5", len(m.Repos))
	}
	names := map[string]bool{}
	for _, r := range m.Repos {
		if r.Name == "" || r.Owner == "" {
			t.Errorf("repo missing name/owner: %+v", r)
		}
		if names[r.Name] {
			t.Errorf("duplicate repo name %q", r.Name)
		}
		names[r.Name] = true
	}
}
