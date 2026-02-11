package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// GenerateSDK generates the decentralized SDK components
func (g *Generator) GenerateSDK() error {
	if err := g.generateSharedSDKTypes(); err != nil {
		return fmt.Errorf("failed to generate shared types: %w", err)
	}

	if err := g.generateResourceSDKs(); err != nil {
		return fmt.Errorf("failed to generate resource SDKs: %w", err)
	}

	return nil
}

func (g *Generator) generateSharedSDKTypes() error {
	usedTypes, err := g.collectUsedTypes()
	if err != nil {
		return fmt.Errorf("failed to collect used types: %w", err)
	}

	allFields := g.collectSchemaFields(usedTypes)
	uniqueStructs := common.CollectUniqueStructs(allFields)

	extraFields := g.calculateIgnoredFields()
	g.applyIgnoredFields(uniqueStructs, extraFields)

	data := map[string]interface{}{
		"Structs": uniqueStructs,
		"Package": "common",
	}

	return g.RenderTemplate(
		"shared_types.go.tmpl",
		[]string{"templates/shared.tmpl", "templates/shared_types.go.tmpl"},
		data,
		filepath.Join(g.config.Generator.OutputDir, "internal", "sdk", "common"),
		"types.go",
	)
}

func (g *Generator) generateResourceSDKs() error {
	for _, name := range g.ResourceOrder {
		rd := g.Resources[name]
		if err := g.generateResourceSDK(rd); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateResourceSDK(rd *common.ResourceData) error {
	outputDir := filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Generate types.go
	if err := g.generateResourceSDKTypes(rd, outputDir); err != nil {
		return err
	}

	// Generate client.go
	if err := g.generateResourceSDKClient(rd, outputDir); err != nil {
		return err
	}

	return nil
}

func (g *Generator) generateResourceSDKTypes(rd *common.ResourceData, outputDir string) error {
	data := map[string]interface{}{
		"Resources": []common.ResourceData{*rd},
		"Package":   rd.CleanName,
	}

	return g.RenderTemplate(
		"sdk_types.go.tmpl",
		[]string{"templates/shared.tmpl", "templates/sdk_types.go.tmpl"},
		data,
		outputDir,
		"types.go",
	)
}

func (g *Generator) generateResourceSDKClient(rd *common.ResourceData, outputDir string) error {
	data := map[string]interface{}{
		"Resources": []common.ResourceData{*rd},
		"Package":   rd.CleanName,
	}

	return g.RenderTemplate(
		"sdk_client.go.tmpl",
		[]string{"templates/shared.tmpl", "templates/sdk_client.go.tmpl"},
		data,
		outputDir,
		"client.go",
	)
}

func (g *Generator) collectUsedTypes() (map[string]bool, error) {
	usedTypes := make(map[string]bool)

	// 0. Construct SchemaConfig
	cfg := g.GetSchemaConfig()

	// Helper to collect types recursively
	var collectTypes func([]common.FieldInfo)
	collectTypes = func(fields []common.FieldInfo) {
		for _, f := range fields {
			if f.RefName != "" {
				if !usedTypes[f.RefName] {
					usedTypes[f.RefName] = true
					// Find schema and recurse
					if schemaRef, ok := g.parser.Document().Components.Schemas[f.RefName]; ok {
						if nestedFields, err := common.ExtractFields(cfg, schemaRef, false); err == nil {
							collectTypes(nestedFields)
						}
					}
				}
			}
			if f.ItemSchema != nil {
				collectTypes([]common.FieldInfo{*f.ItemSchema})
			}
			if len(f.Properties) > 0 {
				collectTypes(f.Properties)
			}
		}
	}

	// 1. Collect types from Resources
	// Explicitly add types used in utils.go
	usedTypes["OrderDetails"] = true

	for _, name := range g.ResourceOrder {
		rd := g.Resources[name]
		collectTypes(rd.CreateFields)
		collectTypes(rd.UpdateFields)
		collectTypes(rd.ResponseFields)
	}

	return usedTypes, nil
}

func (g *Generator) collectSchemaFields(usedTypes map[string]bool) []common.FieldInfo {
	var allFields []common.FieldInfo

	// 0. Construct SchemaConfig
	cfg := g.GetSchemaConfig()

	// Collect only used schemas
	schemaNames := make([]string, 0, len(g.parser.Document().Components.Schemas))
	for name := range g.parser.Document().Components.Schemas {
		schemaNames = append(schemaNames, name)
	}
	sort.Strings(schemaNames)

	for _, name := range schemaNames {
		schemaRef := g.parser.Document().Components.Schemas[name]
		if !usedTypes[name] {
			continue
		}

		// Detect if it's an enum (string type with enum values)
		if schemaRef.Value.Type != nil && (*schemaRef.Value.Type)[0] == "string" && len(schemaRef.Value.Enum) > 0 {
			var enumValues []string
			for _, e := range schemaRef.Value.Enum {
				if s, ok := e.(string); ok {
					enumValues = append(enumValues, s)
				}
			}
			allFields = append(allFields, common.FieldInfo{
				RefName: name,
				GoType:  common.TFTypeString,
				Enum:    enumValues,
			})
			continue
		}

		fields, _ := common.ExtractFields(cfg, schemaRef, false)
		allFields = append(allFields, common.FieldInfo{
			RefName:    name,
			GoType:     common.TFTypeObject,
			Properties: fields,
		})
	}
	return allFields
}

func (g *Generator) calculateIgnoredFields() map[string]map[string]common.FieldInfo {
	// Dynamically calculate ignored fields based on merged resource schemas
	extraFields := make(map[string]map[string]common.FieldInfo)

	var scanForExtraFields func([]common.FieldInfo)
	scanForExtraFields = func(fields []common.FieldInfo) {
		for _, f := range fields {
			// recursion first
			if len(f.Properties) > 0 {
				scanForExtraFields(f.Properties)
			}
			if f.ItemSchema != nil {
				scanForExtraFields([]common.FieldInfo{*f.ItemSchema})
			}

			// Check if this field refers to a shared struct via RefName
			targetName := f.RefName
			if targetName == "" && f.ItemSchema != nil {
				targetName = f.ItemSchema.RefName
			}

			if targetName != "" {
				cleanName := targetName
				if extraFields[cleanName] == nil {
					extraFields[cleanName] = make(map[string]common.FieldInfo)
				}

				if len(f.Properties) > 0 {
					for _, prop := range f.Properties {
						extraFields[cleanName][prop.Name] = prop
					}
				}
				if f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0 {
					for _, prop := range f.ItemSchema.Properties {
						extraFields[cleanName][prop.Name] = prop
					}
				}
			}
		}
	}

	for _, name := range g.ResourceOrder {
		rd := g.Resources[name]
		scanForExtraFields(rd.ModelFields)
	}

	return extraFields
}

func (g *Generator) applyIgnoredFields(uniqueStructs []common.FieldInfo, extraFields map[string]map[string]common.FieldInfo) {
	for i, s := range uniqueStructs {
		if expected, ok := extraFields[s.RefName]; ok {
			// Find missing fields
			existing := make(map[string]bool)
			for _, p := range s.Properties {
				existing[p.Name] = true
			}

			var missing []common.FieldInfo
			expectedNames := make([]string, 0, len(expected))
			for name := range expected {
				expectedNames = append(expectedNames, name)
			}
			sort.Strings(expectedNames)

			for _, name := range expectedNames {
				prop := expected[name]
				if !existing[name] {
					// Create ignored field
					p := prop
					// Map to framework types to support Unknown values
					// Do not force "hidden" tags for missing fields.
					// These fields are likely present in Response but not Request schemas,
					// so they should be treated as normal fields (serialized with omitempty)
					// to allow JSON unmarshalling.
					missing = append(missing, p)
				}
			}

			if len(missing) > 0 {
				uniqueStructs[i].Properties = append(uniqueStructs[i].Properties, missing...)
				// Re-sort properties to ensure consistent order
				sort.Slice(uniqueStructs[i].Properties, func(a, b int) bool {
					return uniqueStructs[i].Properties[a].Name < uniqueStructs[i].Properties[b].Name
				})
			}
		}
	}
}
