package slopfred_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

// writeSkill creates a minimal local skill folder at dir/name with a SKILL.md
// and a sibling file, returning the folder path. It is the source a developer
// would point `add` at.
func writeSkill(t *testing.T, dir, name string) string {
	t.Helper()
	folder := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Join(folder, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"SKILL.md":       "---\nname: " + name + "\ndescription: demo\n---\nbody\n",
		"scripts/run.sh": "#!/bin/sh\necho hi\n",
	}
	for rel, content := range files {
		if err := os.WriteFile(filepath.Join(folder, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return folder
}

func TestAddCopiesFolderAndRecordsOrigin(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	src := writeSkill(t, t.TempDir(), "demo")

	res, err := slopfred.Add(src)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if res.Name != "demo" {
		t.Fatalf("Name = %q, want demo", res.Name)
	}

	// The folder lands verbatim at skills/demo/, including sibling files.
	dst := filepath.Join(home, "skills", "demo")
	got, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if err != nil {
		t.Fatalf("copied SKILL.md missing: %v", err)
	}
	want, _ := os.ReadFile(filepath.Join(src, "SKILL.md"))
	if string(got) != string(want) {
		t.Fatalf("SKILL.md not copied verbatim:\n got %q\nwant %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dst, "scripts", "run.sh")); err != nil {
		t.Fatalf("sibling file not copied: %v", err)
	}

	// The manifest records origin: local for the skill.
	m := manifest(t, home)
	skills, ok := m["skills"].(map[string]any)
	if !ok {
		t.Fatalf("manifest has no skills object: %v", m["skills"])
	}
	entry, ok := skills["demo"].(map[string]any)
	if !ok {
		t.Fatalf("manifest missing demo skill: %v", skills)
	}
	if entry["kind"] != "local" {
		t.Fatalf("origin kind = %v, want local", entry["kind"])
	}
}

// writeUpstreamRepo creates a real git repo at a temp dir populated from layout
// (relative path → file content), commits it, and returns the repo path and its
// HEAD commit. It stands in for a skill published in someone else's repo.
func writeUpstreamRepo(t *testing.T, layout map[string]string) (path, commit string) {
	t.Helper()
	path = t.TempDir()
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
	run("init", "-q")
	run("-c", "user.email=up@test", "-c", "user.name=up", "add", "-A")
	run("-c", "user.email=up@test", "-c", "user.name=up", "commit", "-q", "-m", "init")
	commit = trim(gitOut(t, path, "rev-parse", "HEAD"))
	return path, commit
}

func TestAddUpstreamSubpathPinsAndRecordsOrigin(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	// A monorepo of skills; we pull just skills/foo out of it.
	up, commit := writeUpstreamRepo(t, map[string]string{
		"skills/foo/SKILL.md":       "---\nname: foo\ndescription: demo\n---\nbody\n",
		"skills/foo/scripts/run.sh": "#!/bin/sh\necho hi\n",
		"skills/bar/SKILL.md":       "---\nname: bar\n---\n",
		"README.md":                 "top level\n",
	})

	res, err := slopfred.AddUpstream(up, "skills/foo")
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if res.Name != "foo" {
		t.Fatalf("Name = %q, want foo", res.Name)
	}

	// Only the one skill folder lands, verbatim, at skills/foo/.
	dst := filepath.Join(home, "skills", "foo")
	got, err := os.ReadFile(filepath.Join(dst, "SKILL.md"))
	if err != nil {
		t.Fatalf("copied SKILL.md missing: %v", err)
	}
	want, _ := os.ReadFile(filepath.Join(up, "skills", "foo", "SKILL.md"))
	if string(got) != string(want) {
		t.Fatalf("SKILL.md not copied verbatim:\n got %q\nwant %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(dst, "scripts", "run.sh")); err != nil {
		t.Fatalf("sibling file not copied: %v", err)
	}
	// The sibling skill from the monorepo is not pulled.
	if _, err := os.Stat(filepath.Join(home, "skills", "bar")); !os.IsNotExist(err) {
		t.Fatalf("bar should not be pulled; err=%v", err)
	}

	// The manifest records upstream origin with URL, subpath, and pinned commit.
	m := manifest(t, home)
	skills, _ := m["skills"].(map[string]any)
	entry, ok := skills["foo"].(map[string]any)
	if !ok {
		t.Fatalf("manifest missing foo skill: %v", skills)
	}
	if entry["kind"] != "upstream" {
		t.Fatalf("origin kind = %v, want upstream", entry["kind"])
	}
	if entry["url"] != up {
		t.Fatalf("origin url = %v, want %v", entry["url"], up)
	}
	if entry["subpath"] != "skills/foo" {
		t.Fatalf("origin subpath = %v, want skills/foo", entry["subpath"])
	}
	if entry["commit"] != commit {
		t.Fatalf("origin commit = %v, want %v", entry["commit"], commit)
	}
}

func TestAddUpstreamRootNoSubpath(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	// A repo whose root is itself the skill folder.
	up, commit := writeUpstreamRepo(t, map[string]string{
		"SKILL.md": "---\nname: solo\n---\nbody\n",
	})
	// Rename the repo dir so its basename drives the skill name deterministically.
	renamed := filepath.Join(filepath.Dir(up), "solo-skill")
	if err := os.Rename(up, renamed); err != nil {
		t.Fatal(err)
	}

	res, err := slopfred.AddUpstream(renamed, "")
	if err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}
	if res.Name != "solo-skill" {
		t.Fatalf("Name = %q, want solo-skill", res.Name)
	}
	if _, err := os.Stat(filepath.Join(home, "skills", "solo-skill", "SKILL.md")); err != nil {
		t.Fatalf("root skill not copied: %v", err)
	}
	// The folder is stored verbatim as a SKILL.md unit — the clone's own .git
	// must not travel into the store (which is itself a git repo).
	if _, err := os.Stat(filepath.Join(home, "skills", "solo-skill", ".git")); !os.IsNotExist(err) {
		t.Fatalf("clone .git leaked into the store; err=%v", err)
	}

	m := manifest(t, home)
	skills, _ := m["skills"].(map[string]any)
	entry, _ := skills["solo-skill"].(map[string]any)
	if entry["kind"] != "upstream" {
		t.Fatalf("origin kind = %v, want upstream", entry["kind"])
	}
	if entry["commit"] != commit {
		t.Fatalf("origin commit = %v, want %v", entry["commit"], commit)
	}
	if _, hasSub := entry["subpath"]; hasSub {
		t.Fatalf("root add should record no subpath, got %v", entry["subpath"])
	}
}

func TestAddUpstreamRefusesNameCollision(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	src := writeSkill(t, t.TempDir(), "foo")
	if _, err := slopfred.Add(src); err != nil {
		t.Fatalf("Add: %v", err)
	}

	up, _ := writeUpstreamRepo(t, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\n---\nupstream\n",
	})
	if _, err := slopfred.AddUpstream(up, "skills/foo"); err == nil {
		t.Fatal("AddUpstream colliding with existing skill should error")
	}
}

func TestAddUpstreamRejectsMissingSubpath(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	up, _ := writeUpstreamRepo(t, map[string]string{
		"skills/foo/SKILL.md": "---\nname: foo\n---\n",
	})
	if _, err := slopfred.AddUpstream(up, "skills/nope"); err == nil {
		t.Fatal("AddUpstream with a subpath that has no skill should error")
	}
}

func TestAddUpstreamRequiresStore(t *testing.T) {
	sandbox(t) // sets SLOPFRED_HOME but does not init a store
	up, _ := writeUpstreamRepo(t, map[string]string{
		"SKILL.md": "---\nname: solo\n---\n",
	})
	if _, err := slopfred.AddUpstream(up, ""); err == nil {
		t.Fatal("AddUpstream without an initialised store should error")
	}
}

func TestAddRefusesNameCollision(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	home, _ := slopfred.Home()

	base := t.TempDir()
	src := writeSkill(t, base, "demo")
	if _, err := slopfred.Add(src); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	// A second add of the same name must fail rather than clobber.
	other := writeSkill(t, filepath.Join(base, "other"), "demo")
	if _, err := slopfred.Add(other); err == nil {
		t.Fatal("Add of colliding name should error")
	}

	// The original copy is untouched (still the first source's content).
	got, _ := os.ReadFile(filepath.Join(home, "skills", "demo", "SKILL.md"))
	want, _ := os.ReadFile(filepath.Join(src, "SKILL.md"))
	if string(got) != string(want) {
		t.Fatalf("collision clobbered existing skill")
	}
}

func TestAddRejectsNonSkillFolder(t *testing.T) {
	_, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// A folder with no SKILL.md is not a skill.
	bare := filepath.Join(t.TempDir(), "notaskill")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := slopfred.Add(bare); err == nil {
		t.Fatal("Add of folder without SKILL.md should error")
	}
}

func TestAddRequiresStore(t *testing.T) {
	sandbox(t) // sets SLOPFRED_HOME but does not init a store
	src := writeSkill(t, t.TempDir(), "demo")
	if _, err := slopfred.Add(src); err == nil {
		t.Fatal("Add without an initialised store should error")
	}
}
