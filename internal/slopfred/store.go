// Package slopfred is the in-process core API for slopfred: the single
// behavioural seam over which the CLI is a thin wrapper. It owns the canonical
// store (a git working tree at the slopfred home) and the slopfred-owned
// sidecar manifest.
package slopfred

import (
	"os"
	"path/filepath"
)

const (
	// SkillsDir is the store-relative directory holding canonical skill folders.
	SkillsDir = "skills"
	// ManifestFile is the store-relative path of the sidecar manifest.
	ManifestFile = "slopfred.json"
	// LocalDir is the store-relative, git-ignored directory for device-local
	// bookkeeping (e.g. the activation record).
	LocalDir = "local"
	// homeEnv overrides the default store home; used for testing.
	homeEnv = "SLOPFRED_HOME"
)

// Store is a handle to a slopfred canonical store rooted at Root.
type Store struct {
	// Root is the absolute path to the store home (the git working tree).
	Root string
}

// Home resolves the store home: $SLOPFRED_HOME if set, else ~/.slopfred.
func Home() (string, error) {
	if h := os.Getenv(homeEnv); h != "" {
		return filepath.Abs(h)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".slopfred"), nil
}

// SkillsPath returns the absolute path to the store's skills directory.
func (s *Store) SkillsPath() string { return filepath.Join(s.Root, SkillsDir) }

// ManifestPath returns the absolute path to the store's sidecar manifest.
func (s *Store) ManifestPath() string { return filepath.Join(s.Root, ManifestFile) }

// LocalPath returns the absolute path to the store's git-ignored local dir.
func (s *Store) LocalPath() string { return filepath.Join(s.Root, LocalDir) }
