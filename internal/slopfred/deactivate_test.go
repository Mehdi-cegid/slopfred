package slopfred_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

func TestDeactivateRemovesOnlyRecordedFolders(t *testing.T) {
	seedPack(t, "core", "alpha", "beta")
	project := t.TempDir()

	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// A foreign, hand-authored folder sits alongside the placed folders in the
	// same discovery directory. Deactivation must leave it untouched.
	foreign := filepath.Join(project, ".agents", "skills", "manual")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := slopfred.Deactivate("core", "project", project); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	// Every recorded folder is gone from both discovery trees.
	for _, tree := range []string{".agents/skills", ".claude/skills"} {
		for _, skill := range []string{"alpha", "beta"} {
			p := filepath.Join(project, filepath.FromSlash(tree), skill)
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Fatalf("recorded folder %s/%s still present: err=%v", tree, skill, err)
			}
		}
	}

	// The foreign folder is intact.
	got, err := os.ReadFile(filepath.Join(foreign, "SKILL.md"))
	if err != nil || string(got) != "mine\n" {
		t.Fatalf("foreign folder was disturbed: got=%q err=%v", got, err)
	}
}

func TestDeactivateClearsActivationRecord(t *testing.T) {
	home := seedPack(t, "core", "alpha")
	project := t.TempDir()

	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if _, err := slopfred.Deactivate("core", "project", project); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	a := activations(t, home)
	records, ok := a["records"].([]any)
	if !ok {
		// A nil/absent records list is also an empty record.
		if a["records"] != nil {
			t.Fatalf("records = %v, want empty", a["records"])
		}
		return
	}
	if len(records) != 0 {
		t.Fatalf("records = %v, want the deactivated entry dropped", records)
	}
}

func TestDeactivateDropsOnlyMatchingScope(t *testing.T) {
	seedPack(t, "core", "alpha")

	projectA := t.TempDir()
	projectB := t.TempDir()
	if _, err := slopfred.Activate("core", "project", projectA); err != nil {
		t.Fatalf("Activate A: %v", err)
	}
	if _, err := slopfred.Activate("core", "project", projectB); err != nil {
		t.Fatalf("Activate B: %v", err)
	}

	// Deactivating project A must not disturb project B's placement.
	if _, err := slopfred.Deactivate("core", "project", projectA); err != nil {
		t.Fatalf("Deactivate A: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectA, ".agents", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("project A folder not removed: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(projectB, ".agents", "skills", "alpha", "SKILL.md")); err != nil {
		t.Fatalf("project B placement was disturbed: %v", err)
	}
}

func TestDeactivateDoesNotTouchProjectGitignore(t *testing.T) {
	seedPack(t, "core", "alpha")
	project := t.TempDir()
	gi := filepath.Join(project, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if _, err := slopfred.Deactivate("core", "project", project); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}
	got, _ := os.ReadFile(gi)
	if string(got) != "node_modules\n" {
		t.Fatalf("deactivation modified the user's .gitignore: %q", got)
	}
}

func TestDeactivateUnknownActivationErrors(t *testing.T) {
	seedPack(t, "core", "alpha")
	// Nothing activated for this pack/scope yet.
	if _, err := slopfred.Deactivate("core", "project", t.TempDir()); err == nil {
		t.Fatal("deactivating a pack that was never activated should error")
	}
}

func TestDeactivateRequiresStore(t *testing.T) {
	sandbox(t) // sets SLOPFRED_HOME but does not init a store
	if _, err := slopfred.Deactivate("core", "project", t.TempDir()); err == nil {
		t.Fatal("Deactivate without an initialised store should error")
	}
}
