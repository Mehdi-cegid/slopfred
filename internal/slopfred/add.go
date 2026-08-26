package slopfred

import (
	"fmt"
	"os"
	"path/filepath"
)

// originLocal is the Origin.Kind recorded for user-authored skills added from a
// local folder.
const originLocal = "local"

// AddResult reports the observable outcome of Add.
type AddResult struct {
	// Store is the store the skill was added to.
	Store *Store
	// Name is the skill name, derived from the source folder's basename, under
	// which the folder now lives at skills/<Name>/.
	Name string
}

// Add copies a local skill folder verbatim into the canonical store at
// skills/<name>/ and records origin: local for it in the sidecar manifest. The
// skill name is the source folder's basename. The folder is copied unmodified —
// standard frontmatter only, no rewriting — so it stays a faithful SKILL.md
// unit.
//
// Add refuses to overwrite an existing skill of the same name: a collision is
// an error, never a silent clobber, so users never lose a skill by accident.
func Add(src string) (*AddResult, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return nil, fmt.Errorf("add: resolving source path: %w", err)
	}
	fi, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("add: reading source folder: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("add: source %q is not a skill folder", src)
	}
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
		return nil, fmt.Errorf("add: source %q has no SKILL.md", src)
	}

	root, err := Home()
	if err != nil {
		return nil, fmt.Errorf("add: resolving store home: %w", err)
	}
	store := &Store{Root: root}
	if !isGitRepo(root) {
		return nil, fmt.Errorf("add: no slopfred store at %s (run init first)", root)
	}

	name := filepath.Base(src)
	dst := filepath.Join(store.SkillsPath(), name)
	if _, err := os.Stat(dst); err == nil {
		return nil, fmt.Errorf("add: skill %q already exists in the store; remove it first", name)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("add: checking destination: %w", err)
	}

	if err := copyTree(src, dst); err != nil {
		// Leave no partial folder behind if the copy fails midway.
		_ = os.RemoveAll(dst)
		return nil, fmt.Errorf("add: copying skill folder: %w", err)
	}

	if err := store.updateManifest(func(m *Manifest) {
		m.Skills[name] = Origin{Kind: originLocal}
	}); err != nil {
		// Roll the copy back so a manifest failure leaves no orphan folder.
		_ = os.RemoveAll(dst)
		return nil, fmt.Errorf("add: recording origin: %w", err)
	}

	return &AddResult{Store: store, Name: name}, nil
}
