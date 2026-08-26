package slopfred

import (
	"fmt"
)

// syncCommitName and syncCommitEmail identify slopfred as the author of the
// snapshot commits sync records, so a store syncs even on a machine with no
// user-level git identity configured. Real skill history is the user's; these
// commits only capture "sync point" snapshots of the store.
const (
	syncCommitName  = "slopfred"
	syncCommitEmail = "slopfred@localhost"
	syncCommitMsg   = "slopfred sync"
)

// SyncResult reports the observable outcome of Sync.
type SyncResult struct {
	// Store is the store that was synced.
	Store *Store
	// Branch is the store branch pulled and pushed.
	Branch string
	// Committed is true when sync snapshotted local store changes before
	// pushing; false when the working tree already matched HEAD.
	Committed bool
}

// Sync round-trips the canonical store with its configured git remote so a
// developer's skills and pack manifest travel between devices while each
// device keeps its own activations. It snapshots any pending store changes
// (skills/ and the sidecar manifest) into a commit, pulls the remote, then
// pushes — a git pull then push, driven explicitly under the user's control.
//
// The device-local activation record lives under the git-ignored local/ dir, so
// it is never staged, never committed, and never travels: a new machine that
// clones and syncs converges on the same skills and packs but decides its own
// scope placements.
//
// The first sync against an empty remote skips the pull (there is nothing to
// merge) and seeds the remote branch; later syncs pull before pushing so remote
// changes are integrated first.
func Sync() (*SyncResult, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	root := store.Root

	if remoteURL(root, defaultRemote) == "" {
		return nil, fmt.Errorf("sync: no %q remote configured (run init first)", defaultRemote)
	}

	committed, err := snapshot(root)
	if err != nil {
		return nil, err
	}

	branch := currentBranch(root)
	if branch == "" {
		// A store with nothing committed yet (never happens after snapshot
		// succeeds on a store with content) has no branch to sync.
		return nil, fmt.Errorf("sync: store has no commits to sync")
	}

	// Pull first when the remote already has our branch, so remote changes are
	// integrated before we push. On an empty remote there is nothing to pull.
	if remoteHasBranch(root, defaultRemote, branch) {
		hasContent, err := storeHasContent(store)
		if err != nil {
			return nil, err
		}
		if err := integrate(root, branch, hasContent); err != nil {
			return nil, err
		}
	}

	if _, err := git(root, "push", "-u", defaultRemote, branch); err != nil {
		return nil, fmt.Errorf("sync: pushing to remote: %w", err)
	}

	return &SyncResult{Store: store, Branch: branch, Committed: committed}, nil
}

// integrate brings the remote branch into the local store before a push. When
// this device is joining a remote it has never shared history with — a second
// machine that ran init independently rather than cloning — and it has no
// skills or packs of its own yet (localHasContent is false), it adopts the
// remote wholesale with a hard reset: the pristine empty-init store simply
// becomes the shared store. Otherwise it merges, integrating remote changes
// alongside local ones (a genuine content divergence surfaces as a merge
// conflict for the user to resolve, rather than being silently discarded).
func integrate(root, branch string, localHasContent bool) error {
	if _, err := git(root, "fetch", defaultRemote, branch); err != nil {
		return fmt.Errorf("sync: fetching from remote: %w", err)
	}
	remoteRef := defaultRemote + "/" + branch

	if !localHasContent && unrelated(root, "HEAD", remoteRef) {
		if _, err := git(root, "reset", "--hard", remoteRef); err != nil {
			return fmt.Errorf("sync: adopting remote store: %w", err)
		}
		return nil
	}

	if _, err := git(root,
		"-c", "user.name="+syncCommitName,
		"-c", "user.email="+syncCommitEmail,
		"merge", "--no-edit", "--allow-unrelated-histories", remoteRef,
	); err != nil {
		return fmt.Errorf("sync: merging remote changes: %w", err)
	}
	return nil
}

// unrelated reports whether two refs share no common ancestor, i.e. the local
// history and the remote history were created independently rather than one
// descending from the other.
func unrelated(root, a, b string) bool {
	_, err := git(root, "merge-base", a, b)
	return err != nil
}

// storeHasContent reports whether the local store holds any skills or packs of
// its own — the signal that distinguishes a device with real work to merge from
// a pristine empty-init store that may simply adopt a shared remote.
func storeHasContent(s *Store) (bool, error) {
	m, err := s.ReadManifest()
	if err != nil {
		return false, fmt.Errorf("sync: reading manifest: %w", err)
	}
	return len(m.Skills) > 0 || len(m.Packs) > 0, nil
}

// snapshot stages the whole store working tree and commits it when anything
// changed, reporting whether it made a commit. The git-ignored local/ dir is
// excluded by git itself, so activation records are never captured. Staging all
// tracked and untracked (non-ignored) content means a skill added or a pack
// curated since the last sync is captured without the add/pack operations
// having to commit themselves.
func snapshot(root string) (bool, error) {
	if _, err := git(root, "add", "-A"); err != nil {
		return false, fmt.Errorf("sync: staging store changes: %w", err)
	}

	// Nothing staged relative to HEAD (and HEAD exists) means the working tree
	// already matches the last sync point: no commit needed.
	if hasCommits(root) {
		if _, err := git(root, "diff", "--cached", "--quiet"); err == nil {
			return false, nil
		}
	}

	if _, err := git(root,
		"-c", "user.name="+syncCommitName,
		"-c", "user.email="+syncCommitEmail,
		"commit", "-q", "-m", syncCommitMsg,
	); err != nil {
		return false, fmt.Errorf("sync: committing store snapshot: %w", err)
	}
	return true, nil
}
