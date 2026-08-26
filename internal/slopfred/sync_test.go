package slopfred_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

// twoStores stands up two independent store homes wired to one shared bare
// remote, plus a source dir for skill folders. Each store is selected by
// pointing $SLOPFRED_HOME at it via useStore before driving the core, modelling
// two devices that sync the same remote. It returns the two homes and the
// per-test skill source base.
func twoStores(t *testing.T) (homeA, homeB, srcBase string) {
	t.Helper()
	base := t.TempDir()

	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare git init: %v: %s", err, out)
	}

	homeA = filepath.Join(base, "storeA")
	homeB = filepath.Join(base, "storeB")

	useStore(t, homeA)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init storeA: %v", err)
	}
	useStore(t, homeB)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init storeB: %v", err)
	}

	return homeA, homeB, filepath.Join(base, "src")
}

// useStore points $SLOPFRED_HOME at home so subsequent core calls act on that
// store. It is how a test hops between the two devices sharing a remote.
func useStore(t *testing.T, home string) {
	t.Helper()
	t.Setenv("SLOPFRED_HOME", home)
}

// TestSyncRoundTripsSkillsAndPacksButNotActivations is the behavioural proof of
// SLO-7: two stores sharing one remote converge on the same skills and pack
// manifest after sync, while each store's activation record stays device-local
// and never travels.
func TestSyncRoundTripsSkillsAndPacksButNotActivations(t *testing.T) {
	homeA, homeB, srcBase := twoStores(t)

	// Device A: author a skill, curate a pack, activate it locally, then sync.
	useStore(t, homeA)
	if _, err := slopfred.Add(writeSkill(t, srcBase, "alpha")); err != nil {
		t.Fatalf("Add alpha on A: %v", err)
	}
	if _, err := slopfred.CreatePack("core"); err != nil {
		t.Fatalf("CreatePack on A: %v", err)
	}
	if _, err := slopfred.AddRef("core", "alpha"); err != nil {
		t.Fatalf("AddRef on A: %v", err)
	}
	// Activate on A at user scope into a temp home so A has a local activation
	// record that must NOT travel.
	userHomeA := t.TempDir()
	t.Setenv("HOME", userHomeA)
	if _, err := slopfred.Activate("core", "user", ""); err != nil {
		t.Fatalf("Activate on A: %v", err)
	}
	if _, err := slopfred.Sync(); err != nil {
		t.Fatalf("Sync A: %v", err)
	}

	// Device B: sync to receive A's skill and pack.
	useStore(t, homeB)
	if _, err := slopfred.Sync(); err != nil {
		t.Fatalf("Sync B: %v", err)
	}

	// Skill folder round-tripped to B, byte-for-byte.
	wantBody, err := os.ReadFile(filepath.Join(homeA, "skills", "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read A skill: %v", err)
	}
	gotBody, err := os.ReadFile(filepath.Join(homeB, "skills", "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("skill alpha did not travel to B: %v", err)
	}
	if string(gotBody) != string(wantBody) {
		t.Fatalf("skill content diverged: B=%q want %q", gotBody, wantBody)
	}

	// Pack manifest round-tripped: B references alpha in core.
	if refs := packRefs(t, homeB, "core"); len(refs) != 1 || refs[0] != "alpha" {
		t.Fatalf("pack core on B = %v, want [alpha]", refs)
	}

	// Activation record stayed device-local: A has one, B has none.
	if _, err := os.Stat(filepath.Join(homeA, "local", "activations.json")); err != nil {
		t.Fatalf("A lost its local activation record: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeB, "local", "activations.json")); !os.IsNotExist(err) {
		t.Fatalf("activation record travelled to B (err=%v), must stay local", err)
	}

	// Now prove the reverse direction and convergence on a second round: B adds
	// a skill and packs it, syncs; A syncs and sees it.
	useStore(t, homeB)
	if _, err := slopfred.Add(writeSkill(t, srcBase, "beta")); err != nil {
		t.Fatalf("Add beta on B: %v", err)
	}
	if _, err := slopfred.AddRef("core", "beta"); err != nil {
		t.Fatalf("AddRef beta on B: %v", err)
	}
	if _, err := slopfred.Sync(); err != nil {
		t.Fatalf("second Sync B: %v", err)
	}

	useStore(t, homeA)
	if _, err := slopfred.Sync(); err != nil {
		t.Fatalf("second Sync A: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeA, "skills", "beta", "SKILL.md")); err != nil {
		t.Fatalf("skill beta did not travel back to A: %v", err)
	}
	if refs := packRefs(t, homeA, "core"); len(refs) != 2 || refs[0] != "alpha" || refs[1] != "beta" {
		t.Fatalf("pack core on A = %v, want [alpha beta]", refs)
	}
}

// TestSyncRequiresStore proves sync fails honestly when no store is initialised.
func TestSyncRequiresStore(t *testing.T) {
	t.Setenv("SLOPFRED_HOME", filepath.Join(t.TempDir(), "missing"))
	if _, err := slopfred.Sync(); err == nil {
		t.Fatal("Sync with no store should error")
	}
}
