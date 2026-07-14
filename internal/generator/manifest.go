package generator

import (
	"os"
	"path/filepath"

	"github.com/waldur/terraform-provider-waldur-generator/internal/changelog"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// WriteManifest emits provider-manifest.json into the output directory: a stable,
// schema-driven description of every resource/data source and its attributes.
// It is rsynced into the downstream provider repo on release and becomes the
// baseline the next release diffs against to produce a changelog.
func (g *Generator) WriteManifest() error {
	m := changelog.Manifest{
		Provider: g.config.Generator.ProviderName,
		Entities: make(map[string]changelog.Entity, len(g.ResourceOrder)),
	}
	for _, name := range g.ResourceOrder {
		rd := g.Resources[name]
		if rd == nil {
			continue
		}
		kind := "resource"
		if rd.IsDatasourceOnly {
			kind = "data_source"
		}
		m.Entities[name] = changelog.Entity{
			Kind:       kind,
			Attributes: manifestAttributes(rd.ModelFields),
		}
	}
	data, err := m.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(g.config.Generator.OutputDir, "provider-manifest.json"), data, 0o644)
}

// manifestAttributes converts the generator's FieldInfo model into manifest
// attributes, recursing into nested objects and arrays of objects.
func manifestAttributes(fields []common.FieldInfo) []changelog.Attribute {
	out := make([]changelog.Attribute, 0, len(fields))
	for _, f := range fields {
		if f.SchemaSkip {
			continue
		}
		a := changelog.Attribute{
			Name:     f.Name,
			Type:     manifestType(f),
			Required: f.Required,
			Optional: !f.Required && !f.ServerComputed, // Track Optional mutability explicitly
			Computed: f.ServerComputed,
		}
		switch {
		case len(f.Properties) > 0:
			a.Attributes = manifestAttributes(f.Properties)
		case f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0:
			a.Attributes = manifestAttributes(f.ItemSchema.Properties)
		}
		out = append(out, a)
	}
	return out
}

// manifestType records the Terraform framework type, augmented with the element
// type for arrays so a change of element type registers as a breaking change.
func manifestType(f common.FieldInfo) string {
	t := f.GoType
	if t == "" {
		t = f.Type
	}
	if f.Type == "array" && f.ItemType != "" {
		t = t + "<" + f.ItemType + ">"
	}
	return t
}
