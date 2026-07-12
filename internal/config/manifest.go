// Package config loads the repository audit manifest.
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/tazzledazzle/build-agent-smith/internal/domain"
)

// Manifest is the list of repositories to audit.
type Manifest struct {
	Repos []domain.RepoConfig `json:"repos"`
}

// LoadManifest reads a JSON repo manifest from path.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if len(m.Repos) == 0 {
		return nil, fmt.Errorf("load manifest: no repos defined")
	}
	return &m, nil
}
