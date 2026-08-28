package slopfred_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

// commitToRepo applies layout (relative path → content) on top of the repo at
// path, commits it, and returns the new HEAD commit. It stands in for the
// upstream author publishing a newer version of a skill.
func commitToRepo(t *testing.T, path string, layout map[string]string) (commit string) {
	t.Helper()
	for rel, content := range layout {
		full := filepath.Join(path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("-c", "user.email=up@test", "-c", "user.name=up", "add", "-A")
	run("-c", "user.email=up@test", "-c", "user.name=up", "commit", "-q", "-m", "update")
	return trim(gitOut(t, path, "rev-parse", "HEAD"))
}

func TestUpdateMovesCleanPin(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	up, oldCommit := writeUpstreamRepo(t, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\ndescription: v1\n---\nv1 body\n",
	})
	if _, err := slopfred.AddUpstream(up, "skills/foo"); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	// The upstream author publishes a newer version.
	newCommit := commitToRepo(t, up, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\ndescription: v2\n---\nv2 body\n",
	})
	if newCommit == oldCommit {
		t.Fatal("test setup: new commit should differ from old")
	}

	res, err := slopfred.Update("foo")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(res.Updated) != 1 || res.Updated[0].Name != "foo" {
		t.Fatalf("Updated = %+v, want [foo]", res.Updated)
	}

	// The stored skill folder now holds the newer content.
	got, _ := os.ReadFile(filepath.Join(home, "skills", "foo", "SKILL.md"))
	if want := "---\nname: foo\ndescription: v2\n---\nv2 body\n"; string(got) != want {
		t.Fatalf("stored SKILL.md = %q, want %q", got, want)
	}

	// The manifest pin advances to the new commit.
	m := manifest(t, home)
	skills, _ := m["skills"].(map[string]any)
	entry, _ := skills["foo"].(map[string]any)
	if entry["commit"] != newCommit {
		t.Fatalf("pin = %v, want %v", entry["commit"], newCommit)
	}
}

func TestUpdateAllUpdatesUpstreamAndLeavesLocalAlone(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	up, _ := writeUpstreamRepo(t, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\ndescription: v1\n---\n",
	})
	if _, err := slopfred.AddUpstream(up, "skills/foo"); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	// A local skill that must never be touched by update.
	localSrc := writeSkill(t, t.TempDir(), "mine")
	if _, err := slopfred.Add(localSrc); err != nil {
		t.Fatalf("Add: %v", err)
	}
	localBefore, _ := os.ReadFile(filepath.Join(home, "skills", "mine", "SKILL.md"))

	newCommit := commitToRepo(t, up, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\ndescription: v2\n---\n",
	})

	res, err := slopfred.Update("")
	if err != nil {
		t.Fatalf("Update all: %v", err)
	}
	if len(res.Updated) != 1 || res.Updated[0].Name != "foo" {
		t.Fatalf("Updated = %+v, want [foo]", res.Updated)
	}

	m := manifest(t, home)
	skills, _ := m["skills"].(map[string]any)
	if entry, _ := skills["foo"].(map[string]any); entry["commit"] != newCommit {
		t.Fatalf("foo pin = %v, want %v", entry["commit"], newCommit)
	}
	// The local skill's content and manifest entry are untouched.
	localAfter, _ := os.ReadFile(filepath.Join(home, "skills", "mine", "SKILL.md"))
	if string(localAfter) != string(localBefore) {
		t.Fatalf("local skill content changed by update")
	}
	if mine, _ := skills["mine"].(map[string]any); mine["kind"] != "local" {
		t.Fatalf("local skill origin changed: %v", mine)
	}
}

func TestUpdateRefusesDivergedSkill(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	up, oldCommit := writeUpstreamRepo(t, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\ndescription: v1\n---\n",
	})
	if _, err := slopfred.AddUpstream(up, "skills/foo"); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	// The user locally edits the stored upstream skill: it has now diverged.
	stored := filepath.Join(home, "skills", "foo", "SKILL.md")
	edited := "---\nname: foo\ndescription: v1\n---\nMY LOCAL EDIT\n"
	if err := os.WriteFile(stored, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Upstream also advances.
	commitToRepo(t, up, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\ndescription: v2\n---\n",
	})

	if _, err := slopfred.Update("foo"); err == nil {
		t.Fatal("Update of a locally-diverged skill should refuse")
	}

	// The local edit survives and the pin is unchanged.
	got, _ := os.ReadFile(stored)
	if string(got) != edited {
		t.Fatalf("divergent update clobbered local edit: %q", got)
	}
	m := manifest(t, home)
	skills, _ := m["skills"].(map[string]any)
	entry, _ := skills["foo"].(map[string]any)
	if entry["commit"] != oldCommit {
		t.Fatalf("pin moved on refusal: %v, want %v", entry["commit"], oldCommit)
	}
}

func TestUpdateAllRefusesDivergedButUpdatesRest(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	// Two upstream skills from separate repos.
	upClean, _ := writeUpstreamRepo(t, map[string]string{
		"skills/clean/SKILL.md": "---\nname: clean\ndescription: v1\n---\n",
	})
	if _, err := slopfred.AddUpstream(upClean, "skills/clean"); err != nil {
		t.Fatalf("AddUpstream clean: %v", err)
	}
	upDirty, dirtyOld := writeUpstreamRepo(t, map[string]string{
		"skills/dirty/SKILL.md": "---\nname: dirty\ndescription: v1\n---\n",
	})
	if _, err := slopfred.AddUpstream(upDirty, "skills/dirty"); err != nil {
		t.Fatalf("AddUpstream dirty: %v", err)
	}

	// The user locally edits "dirty" so it diverges from its pin.
	dirtyStored := filepath.Join(home, "skills", "dirty", "SKILL.md")
	edited := "---\nname: dirty\ndescription: v1\n---\nLOCAL EDIT\n"
	if err := os.WriteFile(dirtyStored, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// Both upstreams advance.
	cleanNew := commitToRepo(t, upClean, map[string]string{
		"skills/clean/SKILL.md": "---\nname: clean\ndescription: v2\n---\n",
	})
	commitToRepo(t, upDirty, map[string]string{
		"skills/dirty/SKILL.md": "---\nname: dirty\ndescription: v2\n---\n",
	})

	res, err := slopfred.Update("")
	if err != nil {
		t.Fatalf("Update all: %v", err)
	}

	// The clean skill advances; the diverged one is refused, not aborted.
	if len(res.Updated) != 1 || res.Updated[0].Name != "clean" {
		t.Fatalf("Updated = %+v, want [clean]", res.Updated)
	}
	if len(res.Refused) != 1 || res.Refused[0] != "dirty" {
		t.Fatalf("Refused = %v, want [dirty]", res.Refused)
	}

	m := manifest(t, home)
	skills, _ := m["skills"].(map[string]any)
	if entry, _ := skills["clean"].(map[string]any); entry["commit"] != cleanNew {
		t.Fatalf("clean pin = %v, want %v", entry["commit"], cleanNew)
	}
	// The diverged skill's pin and local edit are both untouched.
	if entry, _ := skills["dirty"].(map[string]any); entry["commit"] != dirtyOld {
		t.Fatalf("dirty pin moved on refusal: %v, want %v", entry["commit"], dirtyOld)
	}
	if got, _ := os.ReadFile(dirtyStored); string(got) != edited {
		t.Fatalf("refused update clobbered local edit: %q", got)
	}
}

func TestUpdateNamedLocalSkillIsRejected(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	src := writeSkill(t, t.TempDir(), "mine")
	if _, err := slopfred.Add(src); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := slopfred.Update("mine"); err == nil {
		t.Fatal("Update of a local-origin skill should error")
	}
}

func TestUpdateUnknownSkillIsRejected(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := slopfred.Update("ghost"); err == nil {
		t.Fatal("Update of an unknown skill should error")
	}
}

func TestUpdateRequiresStore(t *testing.T) {
	sandbox(t) // sets SLOPFRED_HOME but does not init a store
	if _, err := slopfred.Update(""); err == nil {
		t.Fatal("Update without an initialised store should error")
	}
}
