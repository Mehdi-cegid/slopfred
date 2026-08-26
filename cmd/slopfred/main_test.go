package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunInitWiring proves the CLI parses `init <remote>` and drives the core:
// behaviour itself is proven at the core seam, so this only checks arg wiring.
func TestRunInitWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	var buf bytes.Buffer
	if err := run([]string{"init", remote}, &buf); err != nil {
		t.Fatalf("run init: %v", err)
	}
	if !strings.Contains(buf.String(), "initialised") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(home, "skills")); err != nil {
		t.Fatalf("init did not create store: %v", err)
	}
}

func TestRunInitRequiresArg(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"init"}, &buf); err == nil {
		t.Fatal("init with no remote should error")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"frobnicate"}, &buf); err == nil {
		t.Fatal("unknown command should error")
	}
}

// TestRunAddWiring proves the CLI parses `add <path>` and drives the core;
// add behaviour itself is proven at the core seam.
func TestRunAddWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	var initBuf bytes.Buffer
	if err := run([]string{"init", remote}, &initBuf); err != nil {
		t.Fatalf("run init: %v", err)
	}

	src := filepath.Join(base, "demo")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := run([]string{"add", src}, &buf); err != nil {
		t.Fatalf("run add: %v", err)
	}
	if !strings.Contains(buf.String(), "added skill") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(home, "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("add did not copy skill: %v", err)
	}
}

// TestRunAddUpstreamWiring proves the CLI recognises a git URL with a #subpath
// and drives slopfred.AddUpstream; upstream behaviour is proven at the core seam.
func TestRunAddUpstreamWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	var initBuf bytes.Buffer
	if err := run([]string{"init", remote}, &initBuf); err != nil {
		t.Fatalf("run init: %v", err)
	}

	// A real upstream repo with a skill under a subpath.
	up := filepath.Join(base, "upstream")
	if err := os.MkdirAll(filepath.Join(up, "skills", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(up, "skills", "foo", "SKILL.md"), []byte("---\nname: foo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=up@test", "-c", "user.name=up", "add", "-A"},
		{"-c", "user.email=up@test", "-c", "user.name=up", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = up
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	arg := "file://" + up + "#skills/foo"
	var buf bytes.Buffer
	if err := run([]string{"add", arg}, &buf); err != nil {
		t.Fatalf("run add upstream: %v", err)
	}
	if !strings.Contains(buf.String(), "upstream") {
		t.Fatalf("unexpected output: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(home, "skills", "foo", "SKILL.md")); err != nil {
		t.Fatalf("upstream add did not copy skill: %v", err)
	}
}

func TestRunAddRequiresArg(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"add"}, &buf); err == nil {
		t.Fatal("add with no path should error")
	}
}

func TestRunNoArgs(t *testing.T) {
	var buf bytes.Buffer
	if err := run(nil, &buf); err == nil {
		t.Fatal("no args should error with usage")
	}
}
