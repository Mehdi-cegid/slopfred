package slopfred

import (
	"encoding/json"
	"os"
)

// Manifest is the slopfred-owned sidecar manifest at the store root. It is
// versioned and synced, and holds pack definitions and per-skill origin. Init
// writes an empty manifest; later operations populate Packs and Skills.
type Manifest struct {
	// Version is the manifest schema version.
	Version int `json:"version"`
	// Packs maps pack name to its ordered list of skill-name references.
	Packs map[string][]string `json:"packs"`
	// Skills maps skill name to its recorded origin.
	Skills map[string]Origin `json:"skills"`
}

// Origin records per-skill provenance: local (user-authored) or upstream
// (pulled from a git URL + optional subpath, pinned to a commit).
type Origin struct {
	// Kind is "local" or "upstream".
	Kind string `json:"kind"`
	// URL is the upstream git URL (upstream only).
	URL string `json:"url,omitempty"`
	// Subpath is the optional subpath within the upstream repo (upstream only).
	Subpath string `json:"subpath,omitempty"`
	// Commit is the pinned upstream commit (upstream only).
	Commit string `json:"commit,omitempty"`
}

// IsUpstream reports whether this origin points to a pinned upstream git
// source. Callers use this semantic query instead of depending on the manifest's
// string discriminator.
func (o Origin) IsUpstream() bool {
	return o.Kind == originUpstream
}

// manifestVersion is the current sidecar manifest schema version.
const manifestVersion = 1

// newManifest returns an empty manifest with no packs and no skill origins.
func newManifest() *Manifest {
	return &Manifest{
		Version: manifestVersion,
		Packs:   map[string][]string{},
		Skills:  map[string]Origin{},
	}
}

// ReadManifest loads and parses the sidecar manifest from the store.
func (s *Store) ReadManifest() (*Manifest, error) {
	data, err := os.ReadFile(s.ManifestPath())
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// updateManifest reads the manifest, applies mutate to it, and writes it back.
// It is the read-modify-write seam every operation that records origin or pack
// data uses, so callers never hand-roll the load/save dance.
func (s *Store) updateManifest(mutate func(*Manifest)) error {
	m, err := s.ReadManifest()
	if err != nil {
		return err
	}
	if m.Packs == nil {
		m.Packs = map[string][]string{}
	}
	if m.Skills == nil {
		m.Skills = map[string]Origin{}
	}
	mutate(m)
	return s.writeManifest(m)
}

// writeManifest serialises the manifest to the store root as pretty JSON.
func (s *Store) writeManifest(m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(s.ManifestPath(), data, 0o644)
}
