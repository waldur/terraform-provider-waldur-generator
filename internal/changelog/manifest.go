// Package changelog captures what the generated Terraform provider exposes
// (resources, data sources and their attributes) as a stable manifest, and
// diffs two manifests to detect breaking changes between schema revisions.
//
// The generated provider lives in a downstream repo and is overwritten wholesale
// on every release, so its git history records nothing about what changed. The
// manifest is emitted alongside the provider and committed downstream, giving the
// next release a baseline to diff against — the Terraform analogue of diffing an
// Ansible module's argument_spec.
package changelog

import (
	"encoding/json"
	"os"
	"sort"
)

// Attribute is one Terraform schema attribute (recursive for nested blocks).
type Attribute struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Required   bool        `json:"required"`
	Optional   bool        `json:"optional"` // Track Optional mutability explicitly
	Computed   bool        `json:"computed"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

// Entity is a resource or data source exposed by the provider.
type Entity struct {
	Kind       string      `json:"kind"` // "resource" or "data_source"
	Attributes []Attribute `json:"attributes"`
}

// Manifest is the full set of entities the generated provider exposes.
type Manifest struct {
	Provider string            `json:"provider"`
	Entities map[string]Entity `json:"entities"`
}

// Sort orders all attribute slices lexicographically so the serialized manifest
// is byte-stable across runs (map keys are already sorted by encoding/json).
func (m *Manifest) Sort() {
	for name, e := range m.Entities {
		sortAttrs(e.Attributes)
		m.Entities[name] = e
	}
}

func sortAttrs(attrs []Attribute) {
	for i := range attrs {
		sortAttrs(attrs[i].Attributes)
	}
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name < attrs[j].Name })
}

// Marshal renders the manifest as deterministic, indented JSON.
func (m *Manifest) Marshal() ([]byte, error) {
	m.Sort()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Load reads a manifest from disk. A missing file yields an empty manifest (not
// an error), so the first release — with no published baseline — diffs cleanly.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Manifest{Entities: map[string]Entity{}}, nil
	}
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, err
	}
	if m.Entities == nil {
		m.Entities = map[string]Entity{}
	}
	return m, nil
}
