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

func TestRunNoArgs(t *testing.T) {
	var buf bytes.Buffer
	if err := run(nil, &buf); err == nil {
		t.Fatal("no args should error with usage")
	}
}
