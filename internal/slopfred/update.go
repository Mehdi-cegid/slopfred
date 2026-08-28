package slopfred

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// UpdatedSkill reports one upstream skill whose pin update moved to a newer
// commit.
type UpdatedSkill struct {
	// Name is the skill name at skills/<Name>/.
	Name string
	// OldCommit is the pin the skill was on before the update.
	OldCommit string
	// NewCommit is the pin the skill now sits on.
	NewCommit string
}

// UpdateResult reports the observable outcome of Update.
type UpdateResult struct {
	// Store is the store whose skills were updated.
	Store *Store
	// Updated lists the upstream skills whose pin advanced to a newer commit.
	// Upstream skills already on their latest commit are left out: nothing was
	// rewritten for them.
	Updated []UpdatedSkill
	// Refused lists upstream skills skipped because their stored copy has
	// diverged from the pinned ref (updating all only); their files and pins are
	// unchanged.
	Refused []string
}

// errDiverged marks an upstream skill whose stored copy no longer matches its
// pinned ref, so update must refuse rather than clobber the user's edits.
var errDiverged = errors.New("local copy has diverged from the pinned upstream ref")

// Update re-pulls upstream skills and advances their pin to the upstream's
// latest commit. A clean (unedited) upstream skill updates in place; an upstream
// skill whose stored copy has diverged from its pinned ref is refused so local
// edits are never clobbered. Local-origin skills are never touched.
//
// When name is non-empty, exactly that skill is updated: it must exist and be
// upstream, and a divergence is a hard error. When name is empty, every eligible
// upstream skill is updated; diverged skills are skipped and reported in
// UpdateResult.Refused rather than aborting the run.
func Update(name string) (*UpdateResult, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	m, err := store.ReadManifest()
	if err != nil {
		return nil, fmt.Errorf("update: reading manifest: %w", err)
	}

	res := &UpdateResult{Store: store}

	if name != "" {
		origin, ok := m.Skills[name]
		if !ok {
			return nil, fmt.Errorf("update: no skill %q in the store", name)
		}
		if origin.Kind != originUpstream {
			return nil, fmt.Errorf("update: skill %q is local; only upstream skills can be updated", name)
		}
		updated, err := store.updateSkill(name, origin)
		if err != nil {
			if errors.Is(err, errDiverged) {
				return nil, fmt.Errorf("update: skill %q %w; edit it under a local origin or revert your changes", name, errDiverged)
			}
			return nil, err
		}
		if updated != origin.Commit {
			res.Updated = append(res.Updated, UpdatedSkill{Name: name, OldCommit: origin.Commit, NewCommit: updated})
		}
		return res, nil
	}

	names := make([]string, 0, len(m.Skills))
	for n := range m.Skills {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		origin := m.Skills[n]
		if origin.Kind != originUpstream {
			continue
		}
		updated, err := store.updateSkill(n, origin)
		if err != nil {
			if errors.Is(err, errDiverged) {
				res.Refused = append(res.Refused, n)
				continue
			}
			return nil, err
		}
		if updated != origin.Commit {
			res.Updated = append(res.Updated, UpdatedSkill{Name: n, OldCommit: origin.Commit, NewCommit: updated})
		}
	}
	return res, nil
}

// updateSkill re-pulls one upstream skill. It clones the origin, verifies the
// stored folder still matches the pinned ref (refusing with errDiverged if not),
// and — when the upstream has advanced — replaces the stored folder with the
// latest content and records the new pin. It returns the commit the skill now
// sits on. The store folder and manifest are left untouched when the skill is
// diverged or already up to date.
func (s *Store) updateSkill(name string, origin Origin) (commit string, err error) {
	tmp, err := os.MkdirTemp("", "slopfred-update-")
	if err != nil {
		return "", fmt.Errorf("update: creating temp clone dir: %w", err)
	}
	defer os.RemoveAll(tmp)

	// Full clone (not the shallow --depth 1 that add uses): update must check
	// out both the old pin and the new HEAD to compare and then advance, which a
	// shallow clone cannot guarantee.
	if _, err := git(tmp, "clone", origin.URL, "repo"); err != nil {
		return "", fmt.Errorf("update: cloning upstream for %q: %w", name, err)
	}
	clone := filepath.Join(tmp, "repo")

	newCommit, err := git(clone, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("update: resolving upstream commit for %q: %w", name, err)
	}

	stored := filepath.Join(s.SkillsPath(), name)

	// Compare the stored folder against the pinned ref: a mismatch means the
	// user edited the stored copy, so we refuse rather than clobber it.
	if _, err := git(clone, "checkout", "-q", origin.Commit); err != nil {
		return "", fmt.Errorf("update: checking out pinned commit for %q: %w", name, err)
	}
	same, err := sameTree(stored, skillDirIn(clone, origin.Subpath))
	if err != nil {
		return "", fmt.Errorf("update: comparing %q against its pin: %w", name, err)
	}
	if !same {
		return "", errDiverged
	}
	if newCommit == origin.Commit {
		return origin.Commit, nil // already on the latest upstream commit
	}

	// Clean and behind: check out the latest content and stage it beside the
	// stored folder without touching the folder yet.
	if _, err := git(clone, "checkout", "-q", newCommit); err != nil {
		return "", fmt.Errorf("update: checking out latest commit for %q: %w", name, err)
	}
	src := skillDirIn(clone, origin.Subpath)
	if _, err := os.Stat(filepath.Join(src, "SKILL.md")); err != nil {
		return "", fmt.Errorf("update: upstream no longer has a skill at %q for %q", origin.Subpath, name)
	}
	staging, err := s.stageSkillFolder(name, src)
	if err != nil {
		return "", fmt.Errorf("update: staging %q: %w", name, err)
	}
	defer os.RemoveAll(staging)

	// Advance the pin first, then swap the folder in. If the swap fails, roll the
	// pin back so the manifest never claims a commit whose content is not stored.
	old := origin.Commit
	origin.Commit = newCommit
	if err := s.updateManifest(func(m *Manifest) {
		m.Skills[name] = origin
	}); err != nil {
		return "", fmt.Errorf("update: recording new pin for %q: %w", name, err)
	}
	if err := swapDir(staging, stored); err != nil {
		origin.Commit = old
		_ = s.updateManifest(func(m *Manifest) { m.Skills[name] = origin })
		return "", fmt.Errorf("update: replacing %q: %w", name, err)
	}
	return newCommit, nil
}

// stageSkillFolder copies the skill content at src into a sibling staging folder
// of the stored skill (dst+".new"), dropping the clone's own .git so the skill
// is staged verbatim as a SKILL.md unit. Staging beside the target keeps it on
// the same filesystem so the later swap is a rename. It returns the staging path
// for the caller to swap in or clean up.
func (s *Store) stageSkillFolder(name, src string) (string, error) {
	staging := filepath.Join(s.SkillsPath(), name) + ".new"
	if err := os.RemoveAll(staging); err != nil {
		return "", err
	}
	if err := copyTree(src, staging); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	if err := os.RemoveAll(filepath.Join(staging, ".git")); err != nil {
		_ = os.RemoveAll(staging)
		return "", err
	}
	return staging, nil
}

// swapDir replaces dst with staging by removing dst and renaming staging into
// its place. Both live in the same directory, so the rename is atomic.
func swapDir(staging, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return os.Rename(staging, dst)
}

// skillDirIn resolves the skill folder inside a clone: the repo root when
// subpath is empty, otherwise the named subpath folder.
func skillDirIn(clone, subpath string) string {
	if subpath == "" {
		return clone
	}
	return filepath.Join(clone, filepath.FromSlash(subpath))
}

// sameTree reports whether directories a and b hold the same files with the
// same contents, ignoring any .git metadata. It is how update tells a clean
// upstream skill (safe to overwrite) from a locally edited one (must refuse).
func sameTree(a, b string) (bool, error) {
	fa, err := treeFiles(a)
	if err != nil {
		return false, err
	}
	fb, err := treeFiles(b)
	if err != nil {
		return false, err
	}
	if len(fa) != len(fb) {
		return false, nil
	}
	for rel, aBytes := range fa {
		bBytes, ok := fb[rel]
		if !ok || string(aBytes) != string(bBytes) {
			return false, nil
		}
	}
	return true, nil
}

// treeFiles reads root into a map of slash-relative path → file contents,
// skipping .git directories so version-control metadata never affects the
// comparison.
func treeFiles(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
