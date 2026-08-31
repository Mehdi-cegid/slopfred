package slopfred

import (
	"fmt"
	"sort"
)

// SkillStatus is one canonical skill and its recorded origin.
type SkillStatus struct {
	Name   string
	Origin Origin
}

// PackStatus is one named pack and its ordered skill references.
type PackStatus struct {
	Name string
	Refs []string
}

// StatusResult is a read-only snapshot of the canonical store and this
// device's activations.
type StatusResult struct {
	Store       *Store
	Skills      []SkillStatus
	Packs       []PackStatus
	Activations []Activation
}

// Status reports everything a user needs to understand the current slopfred
// setup: canonical skills with their origins, pack membership, and the
// device-local activations describing what was placed and where. Map-backed
// store data is sorted by name so both API consumers and CLI output are
// deterministic; activation order remains the order recorded on this device.
func Status() (*StatusResult, error) {
	store, err := openStore()
	if err != nil {
		return nil, err
	}

	manifest, err := store.ReadManifest()
	if err != nil {
		return nil, fmt.Errorf("status: reading manifest: %w", err)
	}
	activations, err := store.ReadActivations()
	if err != nil {
		return nil, fmt.Errorf("status: reading activation record: %w", err)
	}

	result := &StatusResult{Store: store}

	for _, name := range sortedKeys(manifest.Skills) {
		result.Skills = append(result.Skills, SkillStatus{
			Name:   name,
			Origin: manifest.Skills[name],
		})
	}

	for _, name := range sortedKeys(manifest.Packs) {
		result.Packs = append(result.Packs, PackStatus{
			Name: name,
			Refs: append([]string(nil), manifest.Packs[name]...),
		})
	}

	for _, activation := range activations.Records {
		activation.Targets = append([]string(nil), activation.Targets...)
		activation.Folders = append([]string(nil), activation.Folders...)
		result.Activations = append(result.Activations, activation)
	}

	return result, nil
}

// sortedKeys returns a map's string keys in deterministic order.
func sortedKeys[V any](entries map[string]V) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
