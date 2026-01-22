package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// generateDataSource generates a data source file
func (g *Generator) generateDataSource(dataSource *config.DataSource) error {
	tmpl, err := template.New("datasource.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/datasource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse datasource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "datasources", fmt.Sprintf("%s.go", dataSource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	ops := dataSource.OperationIDs()

	// Extract API paths from OpenAPI operations
	listPath := ""
	retrievePath := ""

	// Use list path as primary since it's needed for filtering
	if _, path, _, err := g.parser.GetOperation(ops.List); err == nil {
		listPath = path
	} else if _, retPath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		// Fall back to retrieve path if list doesn't exist
		listPath = retPath
	}

	// Also get retrieve path separately for UUID lookups
	if _, retPath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		retrievePath = retPath
	}

	// Extract query parameters from list operation
	var filterParams []FilterParam
	if operation, _, _, err := g.parser.GetOperation(ops.List); err == nil {
		for _, param := range operation.Parameters {
			if param.Value != nil && param.Value.In == "query" {
				paramName := param.Value.Name

				// Skip pagination, ordering, and API optimization parameters
				if paramName == "page" || paramName == "page_size" || paramName == "o" || paramName == "field" {
					continue
				}

				description := param.Value.Description

				// Determine Terraform type
				tfType := "String" // Default
				if param.Value.Schema != nil && param.Value.Schema.Value != nil {
					// Get the Go type string (e.g. "types.Int64") from the OpenAPI type
					goType := GetGoType(getSchemaType(param.Value.Schema.Value))
					// Convert it to the simple string identifier used in FilterParam
					tfType = GetFilterParamType(goType)
				}

				filterParams = append(filterParams, FilterParam{
					Name:        paramName,
					Type:        tfType,
					Description: description,
				})
			}
		}
	}

	// Extract Response fields from Retrieve operation
	var responseFields []FieldInfo
	if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
		if fields, err := ExtractFields(responseSchema); err == nil {
			responseFields = fields
		}
	} else if responseSchema, err := g.parser.GetOperationResponseSchema(ops.List); err == nil {
		// For list, the schema is usually an array of items. We need the item schema.
		if responseSchema.Value.Type != nil && (*responseSchema.Value.Type)[0] == "array" && responseSchema.Value.Items != nil {
			if fields, err := ExtractFields(responseSchema.Value.Items); err == nil {
				responseFields = fields
			}
		}
	}

	// Look up corresponding Resource to determine if it's an Order resource
	isOrder := false
	for _, res := range g.config.Resources {
		if res.Name == dataSource.Name {
			if res.Plugin == "order" {
				isOrder = true
			}
			break
		}
	}

	// Apply field exclusion for non-order resources
	var filteredResponseFields []FieldInfo
	for _, rf := range responseFields {
		if !isOrder && ExcludedFields[rf.Name] {
			continue
		}
		filteredResponseFields = append(filteredResponseFields, rf)
	}

	// Sort fields for deterministic output
	sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
	sort.Slice(filteredResponseFields, func(i, j int) bool { return filteredResponseFields[i].Name < filteredResponseFields[j].Name })

	// Apply description filling
	for i := range filterParams {
		fp := &filterParams[i]
		fp.Description = GetDefaultDescription(fp.Name, fp.Description)
	}
	FillDescriptions(filteredResponseFields)

	data := map[string]interface{}{
		"Name":           dataSource.Name,
		"Operations":     ops,
		"ListPath":       listPath,
		"RetrievePath":   retrievePath,
		"FilterParams":   filterParams,
		"ResponseFields": filteredResponseFields,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
