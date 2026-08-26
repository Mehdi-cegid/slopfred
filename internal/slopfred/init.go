package slopfred

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultRemote is the git remote name slopfred configures and drives.
const defaultRemote = "origin"

// InitResult reports the observable outcome of Init.
type InitResult struct {
	// Store is the initialised store.
	Store *Store
	// Created is true when Init created a new store, false when it found an
	// existing store and left it in place (re-running init is safe).
	Created bool
}

// Init stands up the canonical store at the resolved home: a git working tree
// with a skills/ directory and an empty sidecar manifest, with remote set to
// the user-supplied URL. Re-running Init on an existing store is safe: it does
// not clobber the manifest or skills, and only re-points the remote.
//
// The store home is $SLOPFRED_HOME if set, else ~/.slopfred.
func Init(remote string) (*InitResult, error) {
	if remote == "" {
		return nil, fmt.Errorf("init: a git remote URL is required")
	}
	root, err := Home()
	if err != nil {
		return nil, fmt.Errorf("init: resolving store home: %w", err)
	}
	store := &Store{Root: root}

	existed := isGitRepo(root)

	if err := os.MkdirAll(store.SkillsPath(), 0o755); err != nil {
		return nil, fmt.Errorf("init: creating skills dir: %w", err)
	}
	if err := os.MkdirAll(store.LocalPath(), 0o755); err != nil {
		return nil, fmt.Errorf("init: creating local dir: %w", err)
	}

	if !existed {
		if _, err := git(root, "init", "-q"); err != nil {
			return nil, fmt.Errorf("init: git init: %w", err)
		}
	}

	if err := ensureManifest(store); err != nil {
		return nil, err
	}
	if err := ensureGitignore(store); err != nil {
		return nil, err
	}
	if err := ensureSkillsKeep(store); err != nil {
		return nil, err
	}
	if err := setRemote(root, defaultRemote, remote); err != nil {
		return nil, fmt.Errorf("init: setting remote: %w", err)
	}

	return &InitResult{Store: store, Created: !existed}, nil
}

// ensureManifest writes an empty sidecar manifest only if one is not already
// present, so re-running init never clobbers pack or origin data.
func ensureManifest(s *Store) error {
	if _, err := os.Stat(s.ManifestPath()); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("init: checking manifest: %w", err)
	}
	if err := s.writeManifest(newManifest()); err != nil {
		return fmt.Errorf("init: writing manifest: %w", err)
	}
	return nil
}

// ensureGitignore makes the device-local dir git-ignored so activation records
// never travel with the store. It is idempotent.
func ensureGitignore(s *Store) error {
	path := filepath.Join(s.Root, ".gitignore")
	line := "/" + LocalDir + "/"
	if data, err := os.ReadFile(path); err == nil {
		for _, l := range splitLines(string(data)) {
			if l == line {
				return nil
			}
		}
		content := string(data)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			content += "\n"
		}
		content += line + "\n"
		return os.WriteFile(path, []byte(content), 0o644)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("init: checking .gitignore: %w", err)
	}
	return os.WriteFile(path, []byte(line+"\n"), 0o644)
}

// ensureSkillsKeep keeps the empty skills/ directory in git history via a
// .gitkeep placeholder, so the store layout travels even before any skill is
// added. It is idempotent.
func ensureSkillsKeep(s *Store) error {
	keep := filepath.Join(s.SkillsPath(), ".gitkeep")
	if _, err := os.Stat(keep); err == nil {
		return nil
	}
	return os.WriteFile(keep, []byte{}, 0o644)
}

// splitLines splits s on newlines, dropping a trailing empty element.
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
