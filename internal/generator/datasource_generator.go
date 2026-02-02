package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// generateDataSource generates a data source file
func (g *Generator) generateDataSource(dataSource *config.DataSource) error {
	tmpl, err := template.New("datasource.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/datasource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse datasource template: %w", err)
	}

	var f *os.File
	var outputPath string

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
	var filterParams []FieldInfo
	if operation, _, _, err := g.parser.GetOperation(ops.List); err == nil {
		for _, paramRef := range operation.Parameters {
			if paramRef.Value == nil {
				continue
			}
			param := paramRef.Value
			if param.In == "query" {
				paramName := param.Name

				// Skip pagination, ordering, and API optimization parameters
				if paramName == "page" || paramName == "page_size" || paramName == "o" || paramName == "field" {
					continue
				}

				if param.Schema != nil && param.Schema.Value != nil {
					typeStr := getSchemaType(param.Schema.Value)
					goType := GetGoType(typeStr)

					// Filter out complex types if any
					if goType == "" || strings.HasPrefix(goType, "types.List") || strings.HasPrefix(goType, "types.Object") {
						continue
					}

					// Extract enum values
					var enumValues []string
					if len(param.Schema.Value.Enum) > 0 {
						for _, e := range param.Schema.Value.Enum {
							if str, ok := e.(string); ok {
								enumValues = append(enumValues, str)
							}
						}
					}

					filterParams = append(filterParams, FieldInfo{
						Name:        param.Name,
						Type:        typeStr,
						Description: param.Description,
						GoType:      goType,
						Required:    false,
						Enum:        enumValues,
					})
				}
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

	// For datasources, include ALL response fields (no exclusion)
	// since the Terraform model needs all fields for state mapping
	var filteredResponseFields []FieldInfo
	for _, rf := range responseFields {
		filteredResponseFields = append(filteredResponseFields, rf)
	}

	// Sort fields for deterministic output
	sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
	sort.Slice(filteredResponseFields, func(i, j int) bool { return filteredResponseFields[i].Name < filteredResponseFields[j].Name })

	// Apply description filling
	for i := range filterParams {
		fp := &filterParams[i]
		fp.Description = GetDefaultDescription(fp.Name, humanize(dataSource.Name), fp.Description)
	}
	FillDescriptions(filteredResponseFields, humanize(dataSource.Name))

	service, cleanName := splitResourceName(dataSource.Name)

	// If no corresponding resource exists, we need to generate the model file
	// because datasource.go is using shared models
	if !g.hasResource(dataSource.Name) {
		resData := &ResourceData{
			Name:             dataSource.Name,
			Service:          service,
			CleanName:        cleanName,
			ResponseFields:   filteredResponseFields,
			ModelFields:      filteredResponseFields,
			FilterParams:     filterParams,
			IsDatasourceOnly: true,
		}
		if err := g.generateModel(resData); err != nil {
			return fmt.Errorf("failed to generate model for datasource-only %s: %w", dataSource.Name, err)
		}
		// Also generate types.go and client.go for the datasource-only resources
		if err := g.generateResourceSDK(resData); err != nil {
			return fmt.Errorf("failed to generate SDK for datasource %s: %w", dataSource.Name, err)
		}
	}

	data := map[string]interface{}{
		"Name":           dataSource.Name,
		"Service":        service,
		"CleanName":      cleanName,
		"Operations":     ops,
		"ListPath":       listPath,
		"RetrievePath":   retrievePath,
		"FilterParams":   filterParams,
		"ResponseFields": filteredResponseFields,
		"ModelFields":    filteredResponseFields,
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "services", service, cleanName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	outputPath = filepath.Join(outputDir, "datasource.go")
	f, err = os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
