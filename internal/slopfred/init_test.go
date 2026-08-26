package slopfred_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

// sandbox sets $SLOPFRED_HOME to a temp store home and returns that home plus a
// real local bare git remote to wire the store against.
func sandbox(t *testing.T) (home, remote string) {
	t.Helper()
	base := t.TempDir()
	home = filepath.Join(base, "store")

	remote = filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare git init: %v: %s", err, out)
	}

	t.Setenv("SLOPFRED_HOME", home)
	return home, remote
}

// manifest reads and parses the sidecar manifest at home.
func manifest(t *testing.T, home string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "slopfred.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return m
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func TestInitCreatesStoreLayout(t *testing.T) {
	home, remote := sandbox(t)

	res, err := slopfred.Init(remote)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !res.Created {
		t.Fatalf("Created = false on fresh init, want true")
	}
	if res.Store.Root != home {
		t.Fatalf("store root = %q, want %q", res.Store.Root, home)
	}

	// Store home is a git working tree.
	if _, err := os.Stat(filepath.Join(home, ".git")); err != nil {
		t.Fatalf("store is not a git repo: %v", err)
	}
	// skills/ directory exists.
	if fi, err := os.Stat(filepath.Join(home, "skills")); err != nil || !fi.IsDir() {
		t.Fatalf("skills dir missing: err=%v", err)
	}
	// Sidecar manifest present, empty packs + empty skill origins.
	m := manifest(t, home)
	if packs, ok := m["packs"].(map[string]any); !ok || len(packs) != 0 {
		t.Fatalf("manifest packs = %v, want empty object", m["packs"])
	}
	if skills, ok := m["skills"].(map[string]any); !ok || len(skills) != 0 {
		t.Fatalf("manifest skills = %v, want empty object", m["skills"])
	}
}

func TestInitWiresRemote(t *testing.T) {
	home, remote := sandbox(t)

	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got := gitOut(t, home, "remote", "get-url", "origin")
	if trimmed := trim(got); trimmed != remote {
		t.Fatalf("origin url = %q, want %q", trimmed, remote)
	}
}

func TestInitIsIdempotent(t *testing.T) {
	home, remote := sandbox(t)

	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	// Seed a skill folder and mutate the manifest to prove re-init preserves them.
	skill := filepath.Join(home, "skills", "demo")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mangled := `{"version":1,"packs":{"p":["demo"]},"skills":{"demo":{"kind":"local"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(home, "slopfred.json"), []byte(mangled), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-run init with a different remote.
	other := remote + "2"
	if out, err := exec.Command("git", "init", "--bare", "-q", other).CombinedOutput(); err != nil {
		t.Fatalf("second bare init: %v: %s", err, out)
	}
	res, err := slopfred.Init(other)
	if err != nil {
		t.Fatalf("re-init: %v", err)
	}
	if res.Created {
		t.Fatalf("Created = true on re-init, want false")
	}

	// Skill folder untouched.
	if _, err := os.Stat(filepath.Join(skill, "SKILL.md")); err != nil {
		t.Fatalf("re-init clobbered existing skill: %v", err)
	}
	// Manifest not clobbered (pack still present).
	m := manifest(t, home)
	packs, _ := m["packs"].(map[string]any)
	if _, ok := packs["p"]; !ok {
		t.Fatalf("re-init clobbered manifest packs: %v", m["packs"])
	}
	// Remote re-pointed to the new URL.
	if trim(gitOut(t, home, "remote", "get-url", "origin")) != other {
		t.Fatalf("re-init did not update remote")
	}
}

func TestInitRequiresRemote(t *testing.T) {
	sandbox(t)
	if _, err := slopfred.Init(""); err == nil {
		t.Fatal("Init with empty remote should error")
	}
}

func TestInitLocalDirIsGitIgnored(t *testing.T) {
	home, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// git must treat local/ as ignored so activation records never travel.
	out := gitOut(t, home, "check-ignore", "local/activations.json")
	if trim(out) == "" {
		t.Fatalf("local/ is not git-ignored")
	}
}

func trim(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
