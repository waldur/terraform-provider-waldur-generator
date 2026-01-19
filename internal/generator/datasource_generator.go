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
					TFSDKName:   ToSnakeCase(paramName), // Ensure valid TF attribute name
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

	// Deduplicate: remove ResponseFields that exist in FilterParams
	filterNames := make(map[string]bool)
	for _, fp := range filterParams {
		filterNames[fp.Name] = true
	}
	var dedupedResponseFields []FieldInfo
	for _, rf := range responseFields {
		if !filterNames[rf.Name] {
			dedupedResponseFields = append(dedupedResponseFields, rf)
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

	// Filter out marketplace and other fields for non-order resources
	if !isOrder {
		excludeFields := map[string]bool{
			"marketplace_category_name":      true,
			"marketplace_category_uuid":      true,
			"marketplace_offering_name":      true,
			"marketplace_offering_uuid":      true,
			"marketplace_plan_uuid":          true,
			"marketplace_resource_state":     true,
			"marketplace_resource_uuid":      true,
			"is_limit_based":                 true,
			"is_usage_based":                 true,
			"service_name":                   true,
			"service_settings":               true,
			"service_settings_error_message": true,
			"service_settings_state":         true,
			"service_settings_uuid":          true,
			"project":                        true,
			"project_name":                   true,
			"project_uuid":                   true,
			"customer":                       true,
			"customer_abbreviation":          true,
			"customer_name":                  true,
			"customer_native_name":           true,
			"customer_uuid":                  true,
		}

		var filteredFields []FieldInfo
		for _, f := range dedupedResponseFields {
			if !excludeFields[f.TFSDKName] {
				filteredFields = append(filteredFields, f)
			}
		}
		dedupedResponseFields = filteredFields
	}

	// Sort fields for deterministic output
	sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
	sort.Slice(dedupedResponseFields, func(i, j int) bool { return dedupedResponseFields[i].Name < dedupedResponseFields[j].Name })

	data := map[string]interface{}{
		"Name":           dataSource.Name,
		"Operations":     ops,
		"ListPath":       listPath,
		"RetrievePath":   retrievePath,
		"FilterParams":   filterParams,
		"ResponseFields": dedupedResponseFields, // Use deduped version
		"ModelFields":    dedupedResponseFields, // Map to ModelFields for shared template compatibility
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
