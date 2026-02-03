package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/getkin/kin-openapi/openapi3"
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

		// Detect if it's an enum (string type with enum values)
		if schemaRef.Value.Type != nil && (*schemaRef.Value.Type)[0] == "string" && len(schemaRef.Value.Enum) > 0 {
			var enumValues []string
			for _, e := range schemaRef.Value.Enum {
				if s, ok := e.(string); ok {
					enumValues = append(enumValues, s)
				}
			}
			allFields = append(allFields, FieldInfo{
				RefName: name,
				GoType:  "types.String",
				Enum:    enumValues,
			})
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

	var listOp *openapi3.Operation
	if op, path, _, err := g.parser.GetOperation(ops.List); err == nil {
		listPath = path
		listOp = op
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

	// Extract filter parameters
	var filterParams []FieldInfo
	if listOp != nil {
		for _, paramRef := range listOp.Parameters {
			if paramRef.Value == nil {
				continue
			}
			param := paramRef.Value
			if param.In == "query" {
				paramName := param.Name
				if paramName == "page" || paramName == "page_size" || paramName == "o" || paramName == "field" {
					continue
				}
				if param.Schema != nil {
					typeStr := getSchemaType(param.Schema.Value)
					goType := GetGoType(typeStr)
					if goType != "" && !strings.HasPrefix(goType, "types.List") && !strings.HasPrefix(goType, "types.Object") {
						filterParams = append(filterParams, FieldInfo{
							Name:        param.Name,
							Type:        typeStr,
							Description: param.Description,
							GoType:      goType,
							SchemaSkip:  true,
						})
					}
				}
			}
		}
		sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
	}

	// Use response fields for model
	modelFields := responseFields

	// Sort for deterministic output
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })
	sort.Slice(modelFields, func(i, j int) bool { return modelFields[i].Name < modelFields[j].Name })

	service, cleanName := splitResourceName(dataSource.Name)

	return &ResourceData{
		Name:           dataSource.Name,
		Service:        service,
		CleanName:      cleanName,
		ResponseFields: responseFields,
		ModelFields:    modelFields,
		FilterParams:   filterParams,
		APIPaths: map[string]string{
			"Base":     listPath,
			"Retrieve": retrievePath,
		},
		IsDatasourceOnly: true,
		HasDataSource:    true, // Datasource-only entries are by definition datasources
		Operations:       ops,
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
