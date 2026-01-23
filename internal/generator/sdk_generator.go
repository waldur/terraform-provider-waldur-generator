package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
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
	tmpl, err := template.New("shared_types.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/shared_types.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse shared types template: %w", err)
	}

	usedTypes := make(map[string]bool)

	// Helper to collect types recursively
	var collectTypes func([]FieldInfo)
	collectTypes = func(fields []FieldInfo) {
		for _, f := range fields {
			if f.RefName != "" {
				if !usedTypes[f.RefName] {
					usedTypes[f.RefName] = true
					// Find schema and recurse
					if schemaRef, ok := g.parser.Document().Components.Schemas[f.RefName]; ok {
						if nestedFields, err := ExtractFields(schemaRef); err == nil {
							collectTypes(nestedFields)
						}
					}
				}
			}
			if f.ItemSchema != nil {
				collectTypes([]FieldInfo{*f.ItemSchema})
			}
			if len(f.Properties) > 0 {
				collectTypes(f.Properties)
			}
		}
	}

	// 1. Collect types from Resources
	// Explicitly add types used in utils.go
	usedTypes["OrderDetails"] = true

	for _, res := range g.config.Resources {
		rd, err := g.prepareResourceData(&res)
		if err != nil {
			return err
		}
		collectTypes(rd.CreateFields)
		collectTypes(rd.UpdateFields)
		collectTypes(rd.ResponseFields)
	}

	// 2. Collect types from DataSources
	for _, ds := range g.config.DataSources {
		dd, err := g.prepareDatasourceData(&ds)
		if err != nil {
			return err
		}
		collectTypes(dd.ResponseFields)
	}

	var allFields []FieldInfo

	// Collect only used schemas
	for name, schemaRef := range g.parser.Document().Components.Schemas {
		if !usedTypes[name] {
			continue
		}
		fields, _ := ExtractFields(schemaRef)
		allFields = append(allFields, FieldInfo{
			RefName:    name,
			GoType:     "types.Object",
			Properties: fields,
		})
	}

	uniqueStructs := collectUniqueStructs(allFields)
	data := map[string]interface{}{
		"Structs": uniqueStructs,
		"Package": "common",
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "sdk", "common", "types.go")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (g *Generator) generateResourceSDKs() error {
	mergedResources := make(map[string]*ResourceData)
	var resourceOrder []string

	for _, res := range g.config.Resources {
		rd, err := g.prepareResourceData(&res)
		if err != nil {
			return err
		}
		mergedResources[res.Name] = rd
		resourceOrder = append(resourceOrder, res.Name)
	}

	for _, ds := range g.config.DataSources {
		dd, err := g.prepareDatasourceData(&ds)
		if err != nil {
			return err
		}

		if existing, ok := mergedResources[ds.Name]; ok {
			existing.ResponseFields = mergeFields(existing.ResponseFields, dd.ResponseFields)
			existing.ModelFields = mergeFields(existing.ModelFields, dd.ModelFields)
		} else {
			mergedResources[ds.Name] = dd
			resourceOrder = append(resourceOrder, ds.Name)
		}
	}

	for _, name := range resourceOrder {
		rd := mergedResources[name]
		if err := g.generateResourceSDK(rd); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) generateResourceSDK(rd *ResourceData) error {
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

func (g *Generator) generateResourceSDKTypes(rd *ResourceData, outputDir string) error {
	tmpl, err := template.New("sdk_types.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/sdk_types.go.tmpl")
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Resources": []ResourceData{*rd},
		"Package":   rd.CleanName,
	}

	f, err := os.Create(filepath.Join(outputDir, "types.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func (g *Generator) generateResourceSDKClient(rd *ResourceData, outputDir string) error {
	tmpl, err := template.New("sdk_client.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/sdk_client.go.tmpl")
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"Resources": []ResourceData{*rd},
		"Package":   rd.CleanName,
	}

	f, err := os.Create(filepath.Join(outputDir, "client.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

// prepareDatasourceData creates minimal ResourceData for a datasource-only definition
func (g *Generator) prepareDatasourceData(dataSource *config.DataSource) (*ResourceData, error) {
	ops := dataSource.OperationIDs()

	// Extract API paths from OpenAPI operations
	listPath := ""
	retrievePath := ""

	if _, path, _, err := g.parser.GetOperation(ops.List); err == nil {
		listPath = path
	}

	if _, retPath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		retrievePath = retPath
	}

	// Extract Response fields
	var responseFields []FieldInfo
	if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
		if fields, err := ExtractFields(responseSchema); err == nil {
			responseFields = fields
		}
	} else if responseSchema, err := g.parser.GetOperationResponseSchema(ops.List); err == nil {
		if responseSchema.Value.Type != nil && (*responseSchema.Value.Type)[0] == "array" && responseSchema.Value.Items != nil {
			if fields, err := ExtractFields(responseSchema.Value.Items); err == nil {
				responseFields = fields
			}
		}
	}

	// Sort for deterministic output
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })

	service, cleanName := splitResourceName(dataSource.Name)

	return &ResourceData{
		Name:           dataSource.Name,
		Service:        service,
		CleanName:      cleanName,
		ResponseFields: responseFields,
		ModelFields:    responseFields,
		APIPaths: map[string]string{
			"Base":     listPath,
			"Retrieve": retrievePath,
		},
		IsDatasourceOnly: true,
	}, nil
}

// mergeFields combines two slices of FieldInfo while avoiding duplicates by name
func mergeFields(fields1, fields2 []FieldInfo) []FieldInfo {
	seen := make(map[string]bool)
	var result []FieldInfo

	for _, f := range fields1 {
		if !seen[f.Name] {
			seen[f.Name] = true
			result = append(result, f)
		}
	}

	for _, f := range fields2 {
		if !seen[f.Name] {
			seen[f.Name] = true
			result = append(result, f)
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
