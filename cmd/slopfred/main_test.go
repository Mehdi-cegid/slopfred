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

// TestRunPackWiring proves the CLI parses `pack create/add/list` and drives the
// core; pack behaviour itself is proven at the core seam.
func TestRunPackWiring(t *testing.T) {
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

	// A skill to reference.
	src := filepath.Join(base, "demo")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var addBuf bytes.Buffer
	if err := run([]string{"add", src}, &addBuf); err != nil {
		t.Fatalf("run add: %v", err)
	}

	if err := run([]string{"pack", "create", "core"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run pack create: %v", err)
	}

	var addRefBuf bytes.Buffer
	if err := run([]string{"pack", "add", "core", "demo"}, &addRefBuf); err != nil {
		t.Fatalf("run pack add: %v", err)
	}
	if !strings.Contains(addRefBuf.String(), "demo") {
		t.Fatalf("unexpected pack add output: %q", addRefBuf.String())
	}

	var listBuf bytes.Buffer
	if err := run([]string{"pack", "list"}, &listBuf); err != nil {
		t.Fatalf("run pack list: %v", err)
	}
	if !strings.Contains(listBuf.String(), "core") {
		t.Fatalf("pack list missing core: %q", listBuf.String())
	}

	if err := run([]string{"pack", "remove", "core", "demo"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run pack remove: %v", err)
	}
}

// TestRunActivateWiring proves the CLI parses `activate <pack> --scope project`
// and drives slopfred.Activate; activation behaviour is proven at the core seam.
func TestRunActivateWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	if err := run([]string{"init", remote}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	src := filepath.Join(base, "demo")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"add", src}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run add: %v", err)
	}
	if err := run([]string{"pack", "create", "core"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run pack create: %v", err)
	}
	if err := run([]string{"pack", "add", "core", "demo"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run pack add: %v", err)
	}

	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	// Project scope resolves the working directory; drive it via a chdir.
	cwd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	var buf bytes.Buffer
	if err := run([]string{"activate", "core", "--scope", "project"}, &buf); err != nil {
		t.Fatalf("run activate: %v", err)
	}
	if !strings.Contains(buf.String(), "activated pack") {
		t.Fatalf("unexpected activate output: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo", "SKILL.md")); err != nil {
		t.Fatalf("activate did not place skill: %v", err)
	}
}

func TestRunActivateRequiresScope(t *testing.T) {
	if err := run([]string{"activate", "core"}, &bytes.Buffer{}); err == nil {
		t.Fatal("activate without --scope should error")
	}
}

// TestRunDeactivateWiring proves the CLI parses `deactivate <pack> --scope
// project` and drives slopfred.Deactivate over the working directory, removing
// the folder a prior activate placed; behaviour is proven at the core seam.
func TestRunDeactivateWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	if err := run([]string{"init", remote}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run init: %v", err)
	}
	src := filepath.Join(base, "demo")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: demo\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"add", src}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run add: %v", err)
	}
	if err := run([]string{"pack", "create", "core"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run pack create: %v", err)
	}
	if err := run([]string{"pack", "add", "core", "demo"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run pack add: %v", err)
	}

	project := filepath.Join(base, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	if err := run([]string{"activate", "core", "--scope", "project"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run activate: %v", err)
	}

	var buf bytes.Buffer
	if err := run([]string{"deactivate", "core", "--scope", "project"}, &buf); err != nil {
		t.Fatalf("run deactivate: %v", err)
	}
	if !strings.Contains(buf.String(), "deactivated pack") {
		t.Fatalf("unexpected deactivate output: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "skills", "demo")); !os.IsNotExist(err) {
		t.Fatalf("deactivate did not remove placed skill: err=%v", err)
	}
}

func TestRunDeactivateRequiresScope(t *testing.T) {
	if err := run([]string{"deactivate", "core"}, &bytes.Buffer{}); err == nil {
		t.Fatal("deactivate without --scope should error")
	}
}

func TestRunPackRequiresSubcommand(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"pack"}, &buf); err == nil {
		t.Fatal("pack with no subcommand should error")
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

func TestRunSyncWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	if err := run([]string{"init", remote}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	var buf bytes.Buffer
	if err := run([]string{"sync"}, &buf); err != nil {
		t.Fatalf("run sync: %v", err)
	}
	if !strings.Contains(buf.String(), "synced store") {
		t.Fatalf("unexpected sync output: %q", buf.String())
	}

	// Extra args are rejected.
	if err := run([]string{"sync", "extra"}, &bytes.Buffer{}); err == nil {
		t.Fatal("sync with extra args should error")
	}
}

// TestRunUpdateWiring proves the CLI parses `update` and drives slopfred.Update;
// update behaviour itself is proven at the core seam. With no upstream skills the
// store is already current, so the command reports that and succeeds.
func TestRunUpdateWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	if err := run([]string{"init", remote}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	var buf bytes.Buffer
	if err := run([]string{"update"}, &buf); err != nil {
		t.Fatalf("run update: %v", err)
	}
	if !strings.Contains(buf.String(), "up to date") {
		t.Fatalf("unexpected update output: %q", buf.String())
	}

	// Too many args are rejected.
	if err := run([]string{"update", "a", "b"}, &bytes.Buffer{}); err == nil {
		t.Fatal("update with extra args should error")
	}
}

// TestRunStatusWiring proves the CLI accepts no extra arguments and dispatches
// to the core status operation; status behaviour is proven at the core seam.
func TestRunStatusWiring(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "store")
	remote := filepath.Join(base, "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-q", remote).CombinedOutput(); err != nil {
		t.Fatalf("bare init: %v: %s", err, out)
	}
	t.Setenv("SLOPFRED_HOME", home)

	if err := run([]string{"init", remote}, &bytes.Buffer{}); err != nil {
		t.Fatalf("run init: %v", err)
	}

	var buf bytes.Buffer
	if err := run([]string{"status"}, &buf); err != nil {
		t.Fatalf("run status: %v", err)
	}
	if !strings.Contains(buf.String(), "store: "+home) {
		t.Fatalf("unexpected status output: %q", buf.String())
	}

	if err := run([]string{"status", "extra"}, &bytes.Buffer{}); err == nil {
		t.Fatal("status with extra args should error")
	}
}
