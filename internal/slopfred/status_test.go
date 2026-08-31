package slopfred_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mehdi/slopfred/internal/slopfred"
)

func TestStatusReportsStorePacksAndActivations(t *testing.T) {
	home, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if _, err := slopfred.Add(writeSkill(t, t.TempDir(), "zeta")); err != nil {
		t.Fatalf("Add local: %v", err)
	}
	upstream, commit := writeUpstreamRepo(t, map[string]string{
		"skills/alpha/SKILL.md": "---\nname: alpha\n---\n",
	})
	if _, err := slopfred.AddUpstream(upstream, "skills/alpha"); err != nil {
		t.Fatalf("AddUpstream: %v", err)
	}

	for _, pack := range []string{"tools", "core"} {
		if _, err := slopfred.CreatePack(pack); err != nil {
			t.Fatalf("CreatePack %s: %v", pack, err)
		}
	}
	for _, skill := range []string{"zeta", "alpha"} {
		if _, err := slopfred.AddRef("core", skill); err != nil {
			t.Fatalf("AddRef core %s: %v", skill, err)
		}
	}

	project := t.TempDir()
	if _, err := slopfred.Activate("core", "project", project); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	got, err := slopfred.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if got.Store.Root != home {
		t.Fatalf("Store.Root = %q, want %q", got.Store.Root, home)
	}
	if len(got.Skills) != 2 {
		t.Fatalf("skills = %+v, want two", got.Skills)
	}
	if names := []string{got.Skills[0].Name, got.Skills[1].Name}; !reflect.DeepEqual(names, []string{"alpha", "zeta"}) {
		t.Fatalf("skill names = %v, want sorted [alpha zeta]", names)
	}
	if origin := got.Skills[0].Origin; !origin.IsUpstream() || origin.URL != upstream || origin.Subpath != "skills/alpha" || origin.Commit != commit {
		t.Fatalf("alpha origin = %+v, want upstream URL, subpath, and pin", origin)
	}
	if origin := got.Skills[1].Origin; origin.Kind != "local" {
		t.Fatalf("zeta origin = %+v, want local", origin)
	}

	if len(got.Packs) != 2 {
		t.Fatalf("packs = %+v, want two", got.Packs)
	}
	if names := []string{got.Packs[0].Name, got.Packs[1].Name}; !reflect.DeepEqual(names, []string{"core", "tools"}) {
		t.Fatalf("pack names = %v, want sorted [core tools]", names)
	}
	if !reflect.DeepEqual(got.Packs[0].Refs, []string{"zeta", "alpha"}) {
		t.Fatalf("core refs = %v, want manifest order [zeta alpha]", got.Packs[0].Refs)
	}
	if len(got.Activations) != 1 {
		t.Fatalf("activations = %+v, want one", got.Activations)
	}
	activation := got.Activations[0]
	if activation.Pack != "core" || activation.Scope != "project" {
		t.Fatalf("activation = %+v, want core at project scope", activation)
	}
	wantTargets := []string{
		filepath.Join(project, ".agents", "skills"),
		filepath.Join(project, ".claude", "skills"),
	}
	if !reflect.DeepEqual(activation.Targets, wantTargets) {
		t.Fatalf("activation targets = %v, want %v", activation.Targets, wantTargets)
	}
	if !reflect.DeepEqual(activation.Folders, []string{"zeta", "alpha"}) {
		t.Fatalf("activation folders = %v, want [zeta alpha]", activation.Folders)
	}
}

func TestStatusReportsEmptyInitialisedStore(t *testing.T) {
	home, remote := sandbox(t)
	if _, err := slopfred.Init(remote); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got, err := slopfred.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if got.Store.Root != home || len(got.Skills) != 0 || len(got.Packs) != 0 || len(got.Activations) != 0 {
		t.Fatalf("Status = %+v, want empty store snapshot", got)
	}
}

func TestStatusRequiresStore(t *testing.T) {
	sandbox(t)
	if _, err := slopfred.Status(); err == nil {
		t.Fatal("Status without an initialised store should error")
	}
}
