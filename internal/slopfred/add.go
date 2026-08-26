package slopfred

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// originLocal is the Origin.Kind recorded for user-authored skills added from a
// local folder; originUpstream for skills pulled from an upstream git URL.
const (
	originLocal    = "local"
	originUpstream = "upstream"
)

// AddResult reports the observable outcome of Add and AddUpstream.
type AddResult struct {
	// Store is the store the skill was added to.
	Store *Store
	// Name is the skill name under which the folder now lives at skills/<Name>/.
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
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	return finalizeAdd(store, filepath.Base(src), src, Origin{Kind: originLocal})
}

// AddUpstream pulls one skill folder from an upstream git repo into the
// canonical store at skills/<name>/ and records origin: upstream with the URL,
// the optional subpath, and the resolved pinned commit. The pin fixes the exact
// commit so the skill does not change unexpectedly.
//
// When subpath is empty the repository root is the skill folder and the name is
// derived from the URL's final path component (minus any .git suffix); otherwise
// the named folder at that subpath is pulled and the name is its basename. Like
// Add, AddUpstream refuses to overwrite an existing skill of the same name.
func AddUpstream(url, subpath string) (*AddResult, error) {
	if url == "" {
		return nil, fmt.Errorf("add: an upstream git URL is required")
	}
	store, err := openStore()
	if err != nil {
		return nil, err
	}

	tmp, err := os.MkdirTemp("", "slopfred-upstream-")
	if err != nil {
		return nil, fmt.Errorf("add: creating temp clone dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	if _, err := git(tmp, "clone", "--depth", "1", url, "repo"); err != nil {
		return nil, fmt.Errorf("add: cloning upstream: %w", err)
	}
	clone := filepath.Join(tmp, "repo")

	commit, err := git(clone, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("add: resolving upstream commit: %w", err)
	}

	// Drop the clone's own .git so the skill folder is stored verbatim as a
	// SKILL.md unit — never a nested git repo inside the store. This matters
	// only for the root case (subpath folders never contain the clone's .git).
	if err := os.RemoveAll(filepath.Join(clone, ".git")); err != nil {
		return nil, fmt.Errorf("add: cleaning clone metadata: %w", err)
	}

	skillDir := clone
	name := skillNameFromURL(url)
	if subpath != "" {
		skillDir = filepath.Join(clone, filepath.FromSlash(subpath))
		name = filepath.Base(skillDir)
	}

	return finalizeAdd(store, name, skillDir, Origin{
		Kind:    originUpstream,
		URL:     url,
		Subpath: subpath,
		Commit:  commit,
	})
}

// openStore resolves the store home and refuses if no store has been
// initialised there. Its errors are prefixed store:, not with any one command,
// because every operation (add, pack, …) opens the store through it and must
// surface an honest, command-neutral message.
func openStore() (*Store, error) {
	root, err := Home()
	if err != nil {
		return nil, fmt.Errorf("store: resolving store home: %w", err)
	}
	if !isGitRepo(root) {
		return nil, fmt.Errorf("store: no slopfred store at %s (run init first)", root)
	}
	return &Store{Root: root}, nil
}

// finalizeAdd validates srcDir is a skill folder, copies it verbatim into the
// store at skills/<name>/, and records origin in the manifest. It is the shared
// tail of Add and AddUpstream: it refuses a name collision and rolls the copy
// back if recording origin fails, so a failed add never leaves an orphan folder
// or clobbers an existing skill.
func finalizeAdd(store *Store, name, srcDir string, origin Origin) (*AddResult, error) {
	fi, err := os.Stat(srcDir)
	if err != nil {
		return nil, fmt.Errorf("add: reading source folder: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("add: source %q is not a skill folder", srcDir)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "SKILL.md")); err != nil {
		return nil, fmt.Errorf("add: source %q has no SKILL.md", srcDir)
	}

	dst := filepath.Join(store.SkillsPath(), name)
	if _, err := os.Stat(dst); err == nil {
		return nil, fmt.Errorf("add: skill %q already exists in the store; remove it first", name)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("add: checking destination: %w", err)
	}

	if err := copyTree(srcDir, dst); err != nil {
		// Leave no partial folder behind if the copy fails midway.
		_ = os.RemoveAll(dst)
		return nil, fmt.Errorf("add: copying skill folder: %w", err)
	}

	if err := store.updateManifest(func(m *Manifest) {
		m.Skills[name] = origin
	}); err != nil {
		// Roll the copy back so a manifest failure leaves no orphan folder.
		_ = os.RemoveAll(dst)
		return nil, fmt.Errorf("add: recording origin: %w", err)
	}

	return &AddResult{Store: store, Name: name}, nil
}

// skillNameFromURL derives a skill name from an upstream git URL: its final
// path component with any trailing slash and .git suffix removed. It handles
// both scheme URLs (…/name.git) and scp-like remotes (host:name.git).
func skillNameFromURL(url string) string {
	u := strings.TrimRight(url, "/")
	base := u[strings.LastIndexAny(u, "/:")+1:]
	return strings.TrimSuffix(base, ".git")
}
