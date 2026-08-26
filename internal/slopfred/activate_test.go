package slopfred_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

// seedPack inits a store, adds the named skills, creates pack and references
// them all, returning the store home. It is the common arrangement every
// activation test needs before acting.
func seedPack(t *testing.T, pack string, skills ...string) (home string) {
	t.Helper()
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ = slopfred.Home()

	base := t.TempDir()
	if _, err := slopfred.CreatePack(pack); err != nil {
		t.Fatalf("CreatePack: %v", err)
	}
	for _, s := range skills {
		if _, err := slopfred.Add(writeSkill(t, base, s)); err != nil {
			t.Fatalf("Add %s: %v", s, err)
		}
		if _, err := slopfred.AddRef(pack, s); err != nil {
			t.Fatalf("AddRef %s: %v", s, err)
		}
	}
	return home
}

// activations reads and parses the device-local activation record at home.
func activations(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "local", "activations.json"))
	if err != nil {
		t.Fatalf("read activations: %v", err)
	}
	var a map[string]any
	if err := json.Unmarshal(data, &a); err != nil {
		t.Fatalf("parse activations: %v", err)
	}
	return a
}

func TestActivateUserScopePlacesIntoBothDiscoveryTrees(t *testing.T) {
	seedPack(t, "core", "alpha", "beta")

	// User scope discovery paths hang off a temp home.
	userHome := t.TempDir()
	t.Setenv("HOME", userHome)

	res, err := slopfred.Activate("core", "user", "")
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}

	// Both skills land under both discovery trees.
	for _, tree := range []string{".agents/skills", ".claude/skills"} {
		for _, skill := range []string{"alpha", "beta"} {
			p := filepath.Join(userHome, filepath.FromSlash(tree), skill, "SKILL.md")
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("skill %s not placed at %s: %v", skill, tree, err)
			}
		}
	}
	// It copies, not symlinks.
	fi, err := os.Lstat(filepath.Join(userHome, ".agents", "skills", "alpha"))
	if err != nil {
		t.Fatalf("lstat placed folder: %v", err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("activation created a symlink, want a copy")
	}
	if len(res.Targets) != 2 {
		t.Fatalf("Targets = %v, want two discovery trees", res.Targets)
	}
}

func TestActivateProjectScopePlacesIntoProjectPaths(t *testing.T) {
	seedPack(t, "core", "alpha")

	project := t.TempDir()
	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	for _, tree := range []string{".agents/skills", ".claude/skills"} {
		p := filepath.Join(project, filepath.FromSlash(tree), "alpha", "SKILL.md")
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("skill not placed at %s: %v", tree, err)
		}
	}
}

func TestActivateWritesActivationRecord(t *testing.T) {
	home := seedPack(t, "core", "alpha", "beta")
	project := t.TempDir()

	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	a := activations(t, home)
	records, ok := a["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("records = %v, want one entry", a["records"])
	}
	rec, _ := records[0].(map[string]any)
	if rec["pack"] != "core" {
		t.Fatalf("record pack = %v, want core", rec["pack"])
	}
	if rec["scope"] != "project" {
		t.Fatalf("record scope = %v, want project", rec["scope"])
	}
	folders, _ := rec["folders"].([]any)
	if len(folders) != 2 {
		t.Fatalf("record folders = %v, want alpha+beta", rec["folders"])
	}
	targets, _ := rec["targets"].([]any)
	if len(targets) != 2 {
		t.Fatalf("record targets = %v, want two discovery trees", rec["targets"])
	}
}

func TestActivateRecordIsGitIgnored(t *testing.T) {
	home := seedPack(t, "core", "alpha")
	if _, err := slopfred.Activate("core", "project", t.TempDir()); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	// The store must never track the device-local activation record.
	out := gitOut(t, home, "check-ignore", "local/activations.json")
	if trim(out) == "" {
		t.Fatal("activation record is not git-ignored")
	}
}

func TestActivateRefusesForeignCollision(t *testing.T) {
	seedPack(t, "core", "alpha")
	project := t.TempDir()

	// A folder slopfred did not place already occupies the target name.
	foreign := filepath.Join(project, ".agents", "skills", "alpha")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "SKILL.md"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := slopfred.Activate("core", "project", project); err == nil {
		t.Fatal("activation over a foreign folder should refuse")
	}
	// The foreign content is untouched (no overwrite).
	got, _ := os.ReadFile(filepath.Join(foreign, "SKILL.md"))
	if string(got) != "mine\n" {
		t.Fatalf("foreign folder was overwritten: %q", got)
	}
	// Nothing was placed into the other tree either (refuse is all-or-nothing).
	if _, err := os.Stat(filepath.Join(project, ".claude", "skills", "alpha")); !os.IsNotExist(err) {
		t.Fatalf("partial activation despite collision; err=%v", err)
	}
}

func TestActivateReactivationRefreshesOwnPlacement(t *testing.T) {
	seedPack(t, "core", "alpha")
	project := t.TempDir()

	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("first Activate: %v", err)
	}
	// Re-activating the same pack refreshes folders slopfred itself placed
	// rather than tripping the collision guard.
	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("re-Activate should succeed over own placement: %v", err)
	}
}

func TestActivateDoesNotTouchProjectGitignore(t *testing.T) {
	seedPack(t, "core", "alpha")
	project := t.TempDir()
	gi := filepath.Join(project, ".gitignore")
	if err := os.WriteFile(gi, []byte("node_modules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}
	got, _ := os.ReadFile(gi)
	if string(got) != "node_modules\n" {
		t.Fatalf("activation modified the user's .gitignore: %q", got)
	}
}

func TestActivateRejectsUnknownScope(t *testing.T) {
	seedPack(t, "core", "alpha")
	if _, err := slopfred.Activate("core", "sideways", t.TempDir()); err == nil {
		t.Fatal("unknown scope should error")
	}
}

func TestActivateRejectsUnknownPack(t *testing.T) {
	seedPack(t, "core", "alpha")
	if _, err := slopfred.Activate("ghost", "project", t.TempDir()); err == nil {
		t.Fatal("activating an unknown pack should error")
	}
}

func TestActivateRequiresStore(t *testing.T) {
	sandbox(t) // sets SLOPFRED_HOME but does not init a store
	if _, err := slopfred.Activate("core", "project", t.TempDir()); err == nil {
		t.Fatal("Activate without an initialised store should error")
	}
}
