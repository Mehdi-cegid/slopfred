package slopfred

import (
	"fmt"
	"os"
	"path/filepath"
)

// Scope selects where a pack's skills are placed: user-wide on this device, or
// within a single project directory.
const (
	scopeUser    = "user"
	scopeProject = "project"
)

// discoveryDirs are the tool discovery sub-paths, relative to a scope root, that
// slopfred copies skill folders into. Both the agents and claude conventions are
// populated so skills appear wherever the user's tools already look.
var discoveryDirs = []string{
	filepath.Join(".agents", "skills"),
	filepath.Join(".claude", "skills"),
}

// ActivateResult reports the observable outcome of Activate: the store it acted
// on and the recorded activation (pack, scope, the discovery directories
// written, and the skill folders placed in each).
type ActivateResult struct {
	// Store is the store the pack was activated from.
	Store *Store
	// Activation is the record slopfred appended: pack, scope, targets, folders.
	Activation
}

// Activate copies every skill folder referenced by a pack into the standard tool
// discovery paths for the chosen scope, so the pack's skills appear where the
// developer's tools already look. It copies folders (never symlinks), records
// exactly what it placed in the device-local, git-ignored activation record, and
// refuses-and-warns on a name collision with any folder it did not itself place
// — never overwriting foreign content. It never edits the user's .gitignore.
//
// scope must be "user" or "project". For user scope root is ignored and the
// discovery paths hang off the user's home dir; for project scope root is the
// project directory whose .agents/ and .claude/ trees are populated.
func Activate(pack, scope, root string) (*ActivateResult, error) {
	targets, err := discoveryTargets(scope, root)
	if err != nil {
		return nil, err
	}

	store, err := openStore()
	if err != nil {
		return nil, err
	}

	folders, err := packFolders(store, pack)
	if err != nil {
		return nil, err
	}

	record, err := store.ReadActivations()
	if err != nil {
		return nil, fmt.Errorf("activate: reading activation record: %w", err)
	}

	// Validate every placement before writing anything, so a collision leaves
	// the filesystem untouched (refuse-and-warn, never a partial activation).
	placed := placedFolders(record)
	for _, target := range targets {
		for _, folder := range folders {
			dst := filepath.Join(target, folder)
			if err := checkCollision(dst, placed); err != nil {
				return nil, err
			}
		}
	}

	// Copy each referenced skill folder into every discovery target.
	for _, target := range targets {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return nil, fmt.Errorf("activate: creating discovery dir: %w", err)
		}
		for _, folder := range folders {
			src := filepath.Join(store.SkillsPath(), folder)
			dst := filepath.Join(target, folder)
			if err := os.RemoveAll(dst); err != nil {
				return nil, fmt.Errorf("activate: clearing prior placement: %w", err)
			}
			if err := copyTree(src, dst); err != nil {
				return nil, fmt.Errorf("activate: copying skill %q: %w", folder, err)
			}
		}
	}

	record.Records = append(record.Records, Activation{
		Pack:    pack,
		Scope:   scope,
		Targets: targets,
		Folders: folders,
	})
	if err := store.writeActivations(record); err != nil {
		return nil, fmt.Errorf("activate: writing activation record: %w", err)
	}

	return &ActivateResult{
		Store:      store,
		Activation: record.Records[len(record.Records)-1],
	}, nil
}

// discoveryTargets resolves the absolute discovery directories for scope. User
// scope hangs the standard sub-paths off the user's home dir; project scope off
// the given project root. It rejects an unknown scope, or a project scope with
// no root, so a caller never silently writes to the wrong place.
func discoveryTargets(scope, root string) ([]string, error) {
	var base string
	switch scope {
	case scopeUser:
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("activate: resolving home dir: %w", err)
		}
		base = home
	case scopeProject:
		if root == "" {
			return nil, fmt.Errorf("activate: project scope needs a project directory")
		}
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, fmt.Errorf("activate: resolving project dir: %w", err)
		}
		base = abs
	default:
		return nil, fmt.Errorf("activate: scope must be %q or %q, got %q", scopeUser, scopeProject, scope)
	}
	targets := make([]string, len(discoveryDirs))
	for i, d := range discoveryDirs {
		targets[i] = filepath.Join(base, d)
	}
	return targets, nil
}

// packFolders resolves a pack's ordered skill references into the skill-folder
// names to place. It errors if the pack does not exist, so activating a typo
// fails loudly rather than writing nothing.
func packFolders(store *Store, pack string) ([]string, error) {
	m, err := store.ReadManifest()
	if err != nil {
		return nil, fmt.Errorf("activate: reading manifest: %w", err)
	}
	refs, ok := m.Packs[pack]
	if !ok {
		return nil, fmt.Errorf("activate: no pack %q in the store", pack)
	}
	folders := make([]string, len(refs))
	copy(folders, refs)
	return folders, nil
}

// placedFolders indexes every folder slopfred previously wrote, keyed by its
// absolute destination path, from the activation record. A destination present
// here was placed by slopfred and so may be safely refreshed; one absent but
// existing on disk is foreign and must not be overwritten.
func placedFolders(record *Activations) map[string]bool {
	placed := map[string]bool{}
	for _, r := range record.Records {
		for _, target := range r.Targets {
			for _, folder := range r.Folders {
				placed[filepath.Join(target, folder)] = true
			}
		}
	}
	return placed
}

// checkCollision refuses when dst exists on disk but slopfred did not place it,
// so activation never overwrites a folder some other tool or the user created.
func checkCollision(dst string, placed map[string]bool) error {
	if _, err := os.Lstat(dst); err == nil {
		if !placed[dst] {
			return fmt.Errorf("activate: %q already exists and was not placed by slopfred; refusing to overwrite", dst)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("activate: checking %q: %w", dst, err)
	}
	return nil
}
