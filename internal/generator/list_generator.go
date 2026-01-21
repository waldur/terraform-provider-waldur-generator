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

// generateListResource generates a list resource file
func (g *Generator) generateListResource(resource *config.Resource) error {
	// Determine template name and path
	templateName := "list_resource.go.tmpl"

	tmpl, err := template.New(templateName).Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/list_resource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse list resource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", fmt.Sprintf("%s_list.go", resource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	ops := resource.OperationIDs()

	// Extract API paths from OpenAPI operations
	apiPaths := make(map[string]string)

	// Get path from list operation (used as base path)
	var listOp *openapi3.Operation
	if op, listPath, _, err := g.parser.GetOperation(ops.List); err == nil {
		// Remove trailing slash and {uuid} if present for base path
		// Actually for List, we want the collection path (e.g. /api/openstack-tenants/)
		// api_parser.go usually returns /api/openstack-tenants/ for list.
		apiPaths["Base"] = listPath
		listOp = op
	} else {
		// If no list operation, we can't generate a list resource
		return fmt.Errorf("no list operation found for resource %s (op: %s)", resource.Name, ops.List)
	}

	// Extract filter parameters
	var filterParams []FieldInfo
	if listOp != nil {
		for _, paramRef := range listOp.Parameters {
			if paramRef.Value == nil {
				continue
			}
			param := paramRef.Value
			if param.In == "query" && param.Schema != nil && param.Schema.Value != nil {
				typeStr := getSchemaType(param.Schema.Value)
				goType := GetGoType(typeStr)

				// Skip complex types or unknown types for now
				// Mostly strings, booleans, integers.
				if goType == "" || strings.HasPrefix(goType, "types.List") || strings.HasPrefix(goType, "types.Object") {
					continue
				}

				filterParams = append(filterParams, FieldInfo{
					Name:        param.Name,
					Type:        typeStr,
					Description: param.Description,
					GoType:      goType,
					TFSDKName:   ToSnakeCase(param.Name),
					Required:    false, // Filters are optional
				})
			}
		}
		sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
	}

	// Calculate Fields (Reused from resource_generator.go logic)
	// We need ModelFields and ResponseFields for mapResponseFields template.

	var createFields []FieldInfo

	var responseFields []FieldInfo
	var modelFields []FieldInfo

	isOrder := resource.Plugin == "order"

	if isOrder {
		// Order resource logic
		schemaName := strings.ReplaceAll(resource.OfferingType, ".", "") + "CreateOrderAttributes"

		offeringSchema, err := g.parser.GetSchema(schemaName)
		if err == nil {
			if fields, err := ExtractFields(offeringSchema); err == nil {
				createFields = fields
				for i := range createFields {
					createFields[i].Required = false
				}
			}
		}

		if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		}

		modelFields = MergeOrderFields(createFields, responseFields)

		// Add Termination Attributes
		for _, term := range resource.TerminationAttributes {
			goType := "types.String"
			switch term.Type {
			case "boolean":
				goType = "types.Bool"
			case "integer":
				goType = "types.Int64"
			case "number":
				goType = "types.Float64"
			}

			modelFields = append(modelFields, FieldInfo{
				Name:        term.Name,
				Type:        term.Type,
				Description: "Termination attribute",
				GoType:      goType,
				TFSDKName:   ToSnakeCase(term.Name),
			})
		}
	} else {
		// Standard resource logic
		if resource.LinkOp != "" {
			if createSchema, err := g.parser.GetOperationRequestSchema(resource.LinkOp); err == nil {
				if fields, err := ExtractFields(createSchema); err == nil {
					createFields = fields
				}
			}
			// Add Source/Target manually if needed (simplified compared to resource_generator)
			// Ideally we need exact same mapping.
		} else {
			createOp := ops.Create
			if resource.CreateOperation != nil && resource.CreateOperation.OperationID != "" {
				createOp = resource.CreateOperation.OperationID
			}

			if createSchema, err := g.parser.GetOperationRequestSchema(createOp); err == nil {
				if fields, err := ExtractFields(createSchema); err == nil {
					createFields = fields
				}
			}
		}

		if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		} else if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Create); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		}

		allFields := MergeFields(createFields, responseFields)

		if !isOrder {
			// Create a set of input fields to protect them from removal
			inputFields := make(map[string]bool)
			for _, f := range createFields {
				inputFields[f.TFSDKName] = true
			}

			modelFields = make([]FieldInfo, 0)
			for _, f := range allFields {
				if ExcludedFields[f.TFSDKName] && !inputFields[f.TFSDKName] {
					continue
				}
				modelFields = append(modelFields, f)
			}
		} else {
			modelFields = allFields
		}
	}

	// Path params for custom create operation
	if resource.CreateOperation != nil && len(resource.CreateOperation.PathParams) > 0 {
		pathParams := make(map[string]bool)
		for _, v := range resource.CreateOperation.PathParams {
			pathParams[v] = true
		}
		for i, f := range modelFields {
			if pathParams[f.Name] {
				// Ensure required/writeable in model
				modelFields[i].Required = true
				modelFields[i].ReadOnly = false
			}
		}
	}

	// Update responseFields
	modelMap := make(map[string]FieldInfo)
	for _, f := range modelFields {
		modelMap[f.Name] = f
	}
	var newResponseFields []FieldInfo
	for _, f := range responseFields {
		if mergedF, ok := modelMap[f.Name]; ok {
			newResponseFields = append(newResponseFields, mergedF)
		} else {
			newResponseFields = append(newResponseFields, f)
		}
	}
	responseFields = newResponseFields

	// Sort
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })
	sort.Slice(modelFields, func(i, j int) bool { return modelFields[i].Name < modelFields[j].Name })

	FillDescriptions(responseFields)
	FillDescriptions(modelFields)

	data := map[string]interface{}{
		"Name":              resource.Name,
		"APIPaths":          apiPaths,
		"ResponseFields":    responseFields,
		"ModelFields":       modelFields,
		"FilterParams":      filterParams,
		"ProviderName":      g.config.Generator.ProviderName,
		"SkipFilterMapping": true,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
