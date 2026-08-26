package slopfred_test

import (
	"os"
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
