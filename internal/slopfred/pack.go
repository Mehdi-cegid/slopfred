package slopfred

import (
	"fmt"
	"sort"
)

// PackResult reports the observable outcome of a pack curation operation: the
// pack's name and its ordered list of skill-name references after the change.
type PackResult struct {
	// Store is the store the pack lives in.
	Store *Store
	// Name is the pack name.
	Name string
	// Refs is the pack's ordered list of skill-name references.
	Refs []string
}

// CreatePack creates a named, empty pack in the sidecar manifest. A pack is a
// manifest entry, not a container: it starts as an empty ordered list of skill
// references. Creating a pack whose name already exists is an error, never a
// silent reset, so a stray create never wipes an existing pack's contents.
func CreatePack(name string) (*PackResult, error) {
	if name == "" {
		return nil, fmt.Errorf("pack: a pack name is required")
	}
	return curate(name, func(m *Manifest) ([]string, error) {
		if _, exists := m.Packs[name]; exists {
			return nil, fmt.Errorf("pack: pack %q already exists", name)
		}
		m.Packs[name] = []string{}
		return m.Packs[name], nil
	})
}

// AddRef adds a skill reference to a pack by name. The skill must already exist
// in the canonical store — a pack references skills, it does not create them —
// so adding a ref to an unknown skill is rejected with a clear message.
// Referencing a skill from many packs never duplicates it on disk: only the
// flat, ordered reference list in the manifest grows. Adding a ref the pack
// already holds is a no-op, keeping the list free of duplicates.
func AddRef(pack, skill string) (*PackResult, error) {
	return curate(pack, func(m *Manifest) ([]string, error) {
		cur, ok := m.Packs[pack]
		if !ok {
			return nil, fmt.Errorf("pack: no pack %q (create it first)", pack)
		}
		if _, known := m.Skills[skill]; !known {
			return nil, fmt.Errorf("pack: skill %q is not in the store; add it first", skill)
		}
		if !contains(cur, skill) {
			m.Packs[pack] = append(cur, skill)
		}
		return m.Packs[pack], nil
	})
}

// RemoveRef removes a skill reference from a pack by name. Only the reference is
// dropped; the skill folder stays put in the canonical store because other packs
// may still reference it (the library model). Removing a ref the pack does not
// hold, or from a pack that does not exist, is an error so a typo never passes
// silently.
func RemoveRef(pack, skill string) (*PackResult, error) {
	return curate(pack, func(m *Manifest) ([]string, error) {
		cur, ok := m.Packs[pack]
		if !ok {
			return nil, fmt.Errorf("pack: no pack %q", pack)
		}
		if !contains(cur, skill) {
			return nil, fmt.Errorf("pack: pack %q does not reference skill %q", pack, skill)
		}
		m.Packs[pack] = remove(cur, skill)
		return m.Packs[pack], nil
	})
}

// curate opens the store and applies mutate to the manifest under the
// read-modify-write seam, threading a mutation error out. Each mutate performs
// all its validation before touching the manifest, so an error means nothing
// changed and the re-serialised manifest is unchanged. It returns the pack's
// resulting refs, and is the shared spine of CreatePack, AddRef, and RemoveRef.
func curate(name string, mutate func(*Manifest) ([]string, error)) (*PackResult, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	var (
		refs   []string
		mutErr error
	)
	if err := store.updateManifest(func(m *Manifest) {
		refs, mutErr = mutate(m)
	}); err != nil {
		return nil, err
	}
	if mutErr != nil {
		return nil, mutErr
	}
	return &PackResult{Store: store, Name: name, Refs: refs}, nil
}

// ListPacks returns the pack names in the store, sorted for stable output.
func ListPacks() ([]string, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}
	m, err := store.ReadManifest()
	if err != nil {
		return nil, fmt.Errorf("pack: reading manifest: %w", err)
	}
	names := make([]string, 0, len(m.Packs))
	for name := range m.Packs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// contains reports whether refs already holds skill.
func contains(refs []string, skill string) bool {
	for _, r := range refs {
		if r == skill {
			return true
		}
	}
	return false
}

// remove returns refs with the first occurrence of skill dropped, preserving
// order. It allocates a fresh slice so the manifest never shares backing storage
// with a caller's copy.
func remove(refs []string, skill string) []string {
	out := make([]string, 0, len(refs))
	for _, r := range refs {
		if r == skill {
			continue
		}
		out = append(out, r)
	}
	return out
}
