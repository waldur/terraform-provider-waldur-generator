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

	// Deduplicate and handle type compatibility
	filterTypes := make(map[string]string)
	for _, fp := range filterParams {
		filterTypes[fp.Name] = fp.Type
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

	excludeFields := map[string]bool{}
	if !isOrder {
		excludeFields = map[string]bool{
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
		}
	}

	var dedupedResponseFields []FieldInfo
	var mappableFields []FieldInfo // Fields safe to map (type compatible with filters if colliding)

	for _, rf := range responseFields {
		if excludeFields[rf.TFSDKName] {
			continue
		}

		fType, isFilter := filterTypes[rf.Name]
		if !isFilter {
			dedupedResponseFields = append(dedupedResponseFields, rf)
			mappableFields = append(mappableFields, rf)
		} else {
			// Check compatibility
			compatible := false
			switch fType {
			case "String":
				compatible = rf.GoType == "types.String"
			case "Int64":
				compatible = rf.GoType == "types.Int64"
			case "Bool":
				compatible = rf.GoType == "types.Bool"
			case "Float64":
				compatible = rf.GoType == "types.Float64"
			}

			if compatible {
				mappableFields = append(mappableFields, rf)
			}
		}
	}

	// Sort fields for deterministic output
	sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
	sort.Slice(dedupedResponseFields, func(i, j int) bool { return dedupedResponseFields[i].Name < dedupedResponseFields[j].Name })
	sort.Slice(mappableFields, func(i, j int) bool { return mappableFields[i].Name < mappableFields[j].Name })
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })

	// Apply description filling to all field sets
	for i := range filterParams {
		fp := &filterParams[i]
		fp.Description = GetDefaultDescription(fp.Name, fp.Description)
	}

	FillDescriptions(dedupedResponseFields)
	FillDescriptions(mappableFields)
	FillDescriptions(responseFields)

	data := map[string]interface{}{
		"Name":                 dataSource.Name,
		"Operations":           ops,
		"ListPath":             listPath,
		"RetrievePath":         retrievePath,
		"FilterParams":         filterParams,
		"ResponseFields":       responseFields,        // Full list for API Struct
		"ModelFields":          mappableFields,        // Safe list for Mapping
		"UniqueResponseFields": dedupedResponseFields, // Deduped list for Model definition
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
