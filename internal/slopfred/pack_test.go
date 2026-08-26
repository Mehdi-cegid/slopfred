package slopfred_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

// packRefs reads the ordered skill references recorded for pack name in the
// sidecar manifest at home.
func packRefs(t *testing.T, home, name string) []string {
	t.Helper()
	m := manifest(t, home)
	packs, ok := m["packs"].(map[string]any)
	if !ok {
		t.Fatalf("manifest has no packs object: %v", m["packs"])
	}
	raw, ok := packs[name].([]any)
	if !ok {
		t.Fatalf("manifest pack %q is not a list: %v", name, packs[name])
	}
	refs := make([]string, len(raw))
	for i, r := range raw {
		refs[i], _ = r.(string)
	}
	return refs
}

func TestPackCurationRecordsRefsWithoutDuplicatingSkills(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	// Two skills exist in the store.
	base := t.TempDir()
	if _, err := slopfred.Add(writeSkill(t, base, "alpha")); err != nil {
		t.Fatalf("Add alpha: %v", err)
	}
	if _, err := slopfred.Add(writeSkill(t, base, "beta")); err != nil {
		t.Fatalf("Add beta: %v", err)
	}

	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	// A freshly created pack is an empty ordered list.
	if refs := packRefs(t, home, "core"); len(refs) != 0 {
		t.Fatalf("new pack refs = %v, want empty", refs)
	}

	if _, err := slopfred.AddRef("core", "alpha"); err != nil {
		t.Fatalf("AddRef alpha: %v", err)
	}
	if _, err := slopfred.AddRef("core", "beta"); err != nil {
		t.Fatalf("AddRef beta: %v", err)
	}

	// The pack is a flat, ordered list of references in insertion order.
	if refs := packRefs(t, home, "core"); !equalRefs(refs, []string{"alpha", "beta"}) {
		t.Fatalf("core refs = %v, want [alpha beta]", refs)
	}

	// Referencing the same skill from a second pack must not duplicate the
	// folder under skills/ — the skill lives once (library model).
	if _, err := slopfred.CreatePack("extra"); err != nil {
		t.Fatalf("CreatePack extra: %v", err)
	}
	if _, err := slopfred.AddRef("extra", "alpha"); err != nil {
		t.Fatalf("AddRef extra alpha: %v", err)
	}
	if refs := packRefs(t, home, "extra"); !equalRefs(refs, []string{"alpha"}) {
		t.Fatalf("extra refs = %v, want [alpha]", refs)
	}

	// Exactly one copy of alpha exists on disk, referenced by both packs.
	entries, err := os.ReadDir(filepath.Join(home, "skills"))
	if err != nil {
		t.Fatalf("read skills dir: %v", err)
	}
	var alphaCount int
	for _, e := range entries {
		if e.Name() == "alpha" {
			alphaCount++
		}
	}
	if alphaCount != 1 {
		t.Fatalf("alpha copies on disk = %d, want 1", alphaCount)
	}
}

func TestAddRefIsIdempotent(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	if _, err := slopfred.Add(writeSkill(t, t.TempDir(), "alpha")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	if _, err := slopfred.AddRef("core", "alpha"); err != nil {
		t.Fatalf("first AddRef: %v", err)
	}
	if _, err := slopfred.AddRef("core", "alpha"); err != nil {
		t.Fatalf("second AddRef: %v", err)
	}
	// The ref is not duplicated in the list.
	if refs := packRefs(t, home, "core"); !equalRefs(refs, []string{"alpha"}) {
		t.Fatalf("core refs = %v, want [alpha] (no duplicate)", refs)
	}
}

func TestRemoveRefDropsReferenceButKeepsSkill(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	if _, err := slopfred.Add(writeSkill(t, t.TempDir(), "alpha")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	if _, err := slopfred.AddRef("core", "alpha"); err != nil {
		t.Fatalf("AddRef: %v", err)
	}

	if _, err := slopfred.RemoveRef("core", "alpha"); err != nil {
		t.Fatalf("RemoveRef: %v", err)
	}
	// The reference is gone from the pack.
	if refs := packRefs(t, home, "core"); len(refs) != 0 {
		t.Fatalf("core refs = %v, want empty after remove", refs)
	}
	// The skill folder is untouched in the store.
	if _, err := os.Stat(filepath.Join(home, "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatalf("RemoveRef deleted the skill folder: %v", err)
	}
}

func TestAddRefRejectsUnknownSkill(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	// The skill was never added to the store, so referencing it is rejected.
	if _, err := slopfred.AddRef("core", "ghost"); err == nil {
		t.Fatal("AddRef of a skill not in the store should error")
	}
}

func TestAddRefRejectsUnknownPack(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := slopfred.Add(writeSkill(t, t.TempDir(), "alpha")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slopfred.AddRef("nope", "alpha"); err == nil {
		t.Fatal("AddRef to a pack that does not exist should error")
	}
}

func TestCreatePackRejectsDuplicate(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	if _, err := slopfred.Add(writeSkill(t, t.TempDir(), "alpha")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	if _, err := slopfred.AddRef("core", "alpha"); err != nil {
		t.Fatalf("AddRef: %v", err)
	}
	// Re-creating an existing pack must fail rather than wipe its contents.
	if _, err := slopfred.CreatePack("core"); err == nil {
		t.Fatal("CreatePack of an existing name should error")
	}
	if refs := packRefs(t, home, "core"); !equalRefs(refs, []string{"alpha"}) {
		t.Fatalf("duplicate create wiped pack: refs = %v, want [alpha]", refs)
	}
}

func TestRemoveRefRejectsMissingReference(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	// The pack does not reference this skill, so removing it is an error.
	if _, err := slopfred.RemoveRef("core", "alpha"); err == nil {
		t.Fatal("RemoveRef of an unreferenced skill should error")
	}
}

func TestPackCurationRequiresStore(t *testing.T) {
	sandbox(t) // sets SLOPFRED_HOME but does not init a store
	if _, err := slopfred.CreatePack("core"); err == nil {
		t.Fatal("CreatePack without an initialised store should error")
	}
}

// equalRefs reports whether two ordered ref lists are identical.
func equalRefs(a, b []string) bool {
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
