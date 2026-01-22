package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// GenerateSDK generates the internal/sdk package
func (g *Generator) GenerateSDK() error {
	sdkPath := filepath.Join(g.config.Generator.OutputDir, "internal", "sdk")
	if err := os.MkdirAll(sdkPath, 0755); err != nil {
		return fmt.Errorf("failed to create sdk directory: %w", err)
	}

	if err := g.generateSDKTypes(sdkPath); err != nil {
		return fmt.Errorf("failed to generate types.go: %w", err)
	}

	if err := g.generateSDKClient(sdkPath); err != nil {
		return fmt.Errorf("failed to generate client.go: %w", err)
	}

	return nil
}

func (g *Generator) generateSDKTypes(outputDir string) error {
	tmpl, err := template.New("sdk_types.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/sdk_types.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse sdk types template: %w", err)
	}

	// Collect and merge data from resources and datasources
	mergedResources := make(map[string]*ResourceData)
	var resourceOrder []string

	for _, res := range g.config.Resources {
		rd, err := g.prepareResourceData(&res)
		if err != nil {
			return fmt.Errorf("failed to prepare data for resource %s: %w", res.Name, err)
		}
		mergedResources[res.Name] = rd
		resourceOrder = append(resourceOrder, res.Name)
	}

	for _, ds := range g.config.DataSources {
		dd, err := g.prepareDatasourceData(&ds)
		if err != nil {
			return fmt.Errorf("failed to prepare data for datasource %s: %w", ds.Name, err)
		}

		if existing, ok := mergedResources[ds.Name]; ok {
			// Merge response fields
			existing.ResponseFields = mergeFields(existing.ResponseFields, dd.ResponseFields)
			existing.ModelFields = mergeFields(existing.ModelFields, dd.ModelFields)
		} else {
			mergedResources[ds.Name] = dd
			resourceOrder = append(resourceOrder, ds.Name)
		}
	}

	var resources []ResourceData
	var allFields []FieldInfo
	for _, name := range resourceOrder {
		rd := mergedResources[name]
		resources = append(resources, *rd)
		allFields = append(allFields, rd.CreateFields...)
		allFields = append(allFields, rd.UpdateFields...)
		allFields = append(allFields, rd.ResponseFields...)
	}

	sharedStructs := collectUniqueStructs(allFields)

	data := map[string]interface{}{
		"SharedStructs": sharedStructs,
		"Resources":     resources,
	}

	f, err := os.Create(filepath.Join(outputDir, "types.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}

func (g *Generator) generateSDKClient(outputDir string) error {
	tmpl, err := template.New("sdk_client.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/sdk_client.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse sdk client template: %w", err)
	}

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

	var resources []ResourceData
	for _, name := range resourceOrder {
		resources = append(resources, *mergedResources[name])
	}

	data := map[string]interface{}{
		"Resources": resources,
	}

	f, err := os.Create(filepath.Join(outputDir, "client.go"))
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
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

	return &ResourceData{
		Name:           dataSource.Name,
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
