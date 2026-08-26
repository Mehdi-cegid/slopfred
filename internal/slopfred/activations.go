package slopfred

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ActivationsFile is the local-dir-relative name of the device-local,
// git-ignored activation record (it lives at local/activations.json).
const ActivationsFile = "activations.json"

// Activation is one recorded activation of a pack at a scope on this device. It
// captures exactly what slopfred placed and where, so deactivate and status can
// act on the truth of what was written rather than re-deriving it.
type Activation struct {
	// Pack is the activated pack's name.
	Pack string `json:"pack"`
	// Scope is "user" or "project".
	Scope string `json:"scope"`
	// Targets are the absolute discovery directories the skill folders were
	// copied into (both standard discovery trees for the scope).
	Targets []string `json:"targets"`
	// Folders are the skill-folder names slopfred wrote into each target. These
	// are the exact folders it placed, so removal never touches anything else.
	Folders []string `json:"folders"`
}

// Activations is the device-local activation record: the full list of
// activations slopfred has performed on this device.
type Activations struct {
	// Version is the record schema version.
	Version int `json:"version"`
	// Records is the ordered list of activations.
	Records []Activation `json:"records"`
}

// activationsVersion is the current activation-record schema version.
const activationsVersion = 1

// ActivationsPath returns the absolute path to the store's git-ignored
// activation record at local/activations.json.
func (s *Store) ActivationsPath() string {
	return filepath.Join(s.LocalPath(), ActivationsFile)
}

// ReadActivations loads the activation record, returning an empty record when
// none exists yet so callers never special-case the first activation.
func (s *Store) ReadActivations() (*Activations, error) {
	data, err := os.ReadFile(s.ActivationsPath())
	if os.IsNotExist(err) {
		return &Activations{Version: activationsVersion}, nil
	}
	if err != nil {
		return nil, err
	}
	var a Activations
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// writeActivations serialises the record into the store's git-ignored local dir
// as pretty JSON, creating the dir if needed.
func (s *Store) writeActivations(a *Activations) error {
	if err := os.MkdirAll(s.LocalPath(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(s.ActivationsPath(), data, 0o644)
}
