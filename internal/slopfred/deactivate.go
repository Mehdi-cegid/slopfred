package slopfred

import (
	"fmt"
	"os"
	"path/filepath"
)

// DeactivateResult reports the observable outcome of Deactivate: the store it
// acted on and the activation record it removed (pack, scope, the discovery
// directories it wrote, and the skill folders it deleted from each).
type DeactivateResult struct {
	// Store is the store the pack was deactivated from.
	Store *Store
	// Activation is the record slopfred removed: pack, scope, targets, folders.
	Activation
}

// Deactivate removes exactly the skill folders slopfred placed for one prior
// activation — the pack at the given scope and root — and nothing else. It reads
// the device-local activation record to learn precisely which folders it wrote
// into which discovery directories, deletes only those, and then drops that
// entry from the record. Foreign or hand-authored folders sharing the same
// discovery directory are left intact, so slopfred and manual skills mix safely.
// It never edits the user's .gitignore.
//
// scope must be "user" or "project"; the (scope, root) pair is resolved to the
// same discovery targets Activate used, so the matching activation is found even
// when several projects share a pack. It errors if no such activation exists, so
// deactivating something that was never activated fails loudly.
func Deactivate(pack, scope, root string) (*DeactivateResult, error) {
	targets, err := discoveryTargets(scope, root)
	if err != nil {
		return nil, err
	}

	store, err := openStore()
	if err != nil {
		return nil, err
	}

	record, err := store.ReadActivations()
	if err != nil {
		return nil, fmt.Errorf("deactivate: reading activation record: %w", err)
	}

	idx := findActivation(record, pack, scope, targets)
	if idx < 0 {
		return nil, fmt.Errorf("deactivate: no activation of pack %q at %s scope to remove", pack, scope)
	}
	removed := record.Records[idx]

	// Delete exactly the folders this activation recorded — never a whole
	// discovery directory — so foreign folders beside them survive.
	for _, target := range removed.Targets {
		for _, folder := range removed.Folders {
			dst := filepath.Join(target, folder)
			if err := os.RemoveAll(dst); err != nil {
				return nil, fmt.Errorf("deactivate: removing %q: %w", dst, err)
			}
		}
	}

	record.Records = append(record.Records[:idx], record.Records[idx+1:]...)
	if err := store.writeActivations(record); err != nil {
		return nil, fmt.Errorf("deactivate: writing activation record: %w", err)
	}

	return &DeactivateResult{Store: store, Activation: removed}, nil
}

// findActivation returns the index of the recorded activation matching pack,
// scope, and the resolved discovery targets, or -1 when none matches. Targets
// disambiguate two project-scope activations of the same pack in different
// directories.
func findActivation(record *Activations, pack, scope string, targets []string) int {
	for i, r := range record.Records {
		if r.Pack == pack && r.Scope == scope && sameTargets(r.Targets, targets) {
			return i
		}
	}
	return -1
}

// sameTargets reports whether two target lists hold the same discovery
// directories in the same order.
func sameTargets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
