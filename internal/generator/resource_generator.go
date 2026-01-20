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

// generateResource generates a resource file
func (g *Generator) generateResource(resource *config.Resource) error {
	tmpl, err := template.New("resource.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/resource.go.tmpl", "templates/resource_standard.tmpl", "templates/resource_order.tmpl", "templates/resource_link.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse resource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", fmt.Sprintf("%s.go", resource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	ops := resource.OperationIDs()

	// Extract API paths from OpenAPI operations
	apiPaths := make(map[string]string)

	// Get path from list operation (used as base path)
	if _, listPath, _, err := g.parser.GetOperation(ops.List); err == nil {
		// Remove trailing slash and {uuid} if present for base path
		basePath := listPath
		apiPaths["Base"] = basePath
	}

	// Get path from create operation (check for custom create operation first)
	if resource.CreateOperation != nil && resource.CreateOperation.OperationID != "" {
		if _, createPath, _, err := g.parser.GetOperation(resource.CreateOperation.OperationID); err == nil {
			apiPaths["Create"] = createPath
			apiPaths["CreateOperationID"] = resource.CreateOperation.OperationID
			// Store path params for template
			for k, v := range resource.CreateOperation.PathParams {
				apiPaths["CreatePathParam_"+k] = v
			}
		}
	} else if _, createPath, _, err := g.parser.GetOperation(ops.Create); err == nil {
		apiPaths["Create"] = createPath
	}

	// Get path from retrieve operation (includes UUID parameter)
	if _, retrievePath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		apiPaths["Retrieve"] = retrievePath
	}

	// Get path from update operation
	if _, updatePath, _, err := g.parser.GetOperation(ops.PartialUpdate); err == nil {
		apiPaths["Update"] = updatePath
	}

	// Get path from delete operation
	if _, deletePath, _, err := g.parser.GetOperation(ops.Destroy); err == nil {
		apiPaths["Delete"] = deletePath
	}

	// Link plugin paths
	if resource.LinkOp != "" {
		if _, linkPath, _, err := g.parser.GetOperation(resource.LinkOp); err == nil {
			apiPaths["Link"] = linkPath
		}
	}
	if resource.UnlinkOp != "" {
		if _, unlinkPath, _, err := g.parser.GetOperation(resource.UnlinkOp); err == nil {
			apiPaths["Unlink"] = unlinkPath
		}
	}
	if resource.Source != nil && resource.Source.RetrieveOp != "" {
		if _, sourcePath, _, err := g.parser.GetOperation(resource.Source.RetrieveOp); err == nil {
			apiPaths["SourceRetrieve"] = sourcePath
		}
	}

	// Resolve update action paths from OpenAPI schema
	var updateActions []UpdateAction
	for actionName, actionConfig := range resource.UpdateActions {
		action := UpdateAction{
			Name:       actionName,
			Operation:  actionConfig.Operation,
			Param:      actionConfig.Param,
			CompareKey: actionConfig.CompareKey,
		}
		// Default CompareKey to Param if not specified
		if action.CompareKey == "" {
			action.CompareKey = action.Param
		}
		// Resolve path from OpenAPI operation
		if _, actionPath, _, err := g.parser.GetOperation(actionConfig.Operation); err == nil {
			action.Path = actionPath
		}
		updateActions = append(updateActions, action)
	}

	// Extract fields
	var createFields []FieldInfo
	var updateFields []FieldInfo
	var responseFields []FieldInfo
	var modelFields []FieldInfo

	isOrder := resource.Plugin == "order"

	if isOrder {
		// Order resource logic
		// 1. Get Offering Schema (Input)
		// Remove dots from offering type for schema name (e.g. OpenStack.Instance -> OpenStackInstanceCreateOrderAttributes)
		schemaName := strings.ReplaceAll(resource.OfferingType, ".", "") + "CreateOrderAttributes"

		offeringSchema, err := g.parser.GetSchema(schemaName)
		if err != nil {
			return fmt.Errorf("failed to find offering schema %s: %w", schemaName, err)
		}

		if fields, err := ExtractFields(offeringSchema); err == nil {
			createFields = fields
			// Mark all plugin fields as optional to allow system-populated values
			// and delegate validation to the API
			for i := range createFields {
				createFields[i].Required = false
			}
		}

		// 2. Get Resource Schema (Output) from Retrieve operation
		if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		}

		// 3. Merge fields
		modelFields = MergeOrderFields(createFields, responseFields)

		// 4. Add Termination Attributes
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
				// Required: false, ReadOnly: false -> Optional: true
			})
		}

		// Extract Update fields from Resource PartialUpdate operation
		if updateSchema, err := g.parser.GetOperationRequestSchema(ops.PartialUpdate); err == nil {
			if fields, err := ExtractFields(updateSchema); err == nil {
				updateFields = fields
			}
		}

	} else {
		// Standard resource logic
		// Extract Create fields
		if resource.LinkOp != "" {
			// Link Plugin: Use LinkOp input schema
			if createSchema, err := g.parser.GetOperationRequestSchema(resource.LinkOp); err == nil {
				if fields, err := ExtractFields(createSchema); err == nil {
					createFields = fields
				}
			}
			// Add Source and Target fields manually if not present
			// This ensures UUID/String handling is correct even if not in schema directly

			// Source Param (usually in URL, but needs to be an input)
			if resource.Source != nil && resource.Source.Param != "" {
				// Check if already exists
				found := false
				for _, f := range createFields {
					if f.Name == resource.Source.Param {
						found = true
						break
					}
				}
				if !found {
					createFields = append(createFields, FieldInfo{
						Name:        resource.Source.Param,
						Type:        "string",
						Description: "Source resource UUID",
						GoType:      "types.String",
						TFSDKName:   ToSnakeCase(resource.Source.Param),
						Required:    true,
					})
				}
			}

			// Target Param
			if resource.Target != nil && resource.Target.Param != "" {
				// Check if already exists
				found := false
				for _, f := range createFields {
					if f.Name == resource.Target.Param {
						found = true
						break
					}
				}
				if !found {
					createFields = append(createFields, FieldInfo{
						Name:        resource.Target.Param,
						Type:        "string",
						Description: "Target resource UUID",
						GoType:      "types.String",
						TFSDKName:   ToSnakeCase(resource.Target.Param),
						Required:    true,
					})
				}
			}

			// Additional Link Params (e.g. device)
			for _, param := range resource.LinkParams {
				// Check if already exists
				found := false
				for _, f := range createFields {
					if f.Name == param.Name {
						found = true
						break
					}
				}
				if !found {
					goType := "types.String"
					switch param.Type {
					case "boolean":
						goType = "types.Bool"
					case "integer":
						goType = "types.Int64"
					case "number":
						goType = "types.Float64"
					}
					createFields = append(createFields, FieldInfo{
						Name:        param.Name,
						Type:        param.Type,
						Description: "Link parameter",
						GoType:      goType,
						TFSDKName:   ToSnakeCase(param.Name),
						Required:    false, // Usually optional
					})
				}
			}

		} else {
			// Extract Create fields
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

		// Inject Path Params for Custom Create Operation as strict Input Fields
		if resource.CreateOperation != nil && len(resource.CreateOperation.PathParams) > 0 {
			for _, paramName := range resource.CreateOperation.PathParams {
				// Check if already exists in createFields
				found := false
				for _, f := range createFields {
					if f.Name == paramName {
						found = true
						break
					}
				}
				if !found {
					createFields = append(createFields, FieldInfo{
						Name:        paramName,
						Type:        "string",
						Description: "Required path parameter for resource creation",
						GoType:      "types.String",
						TFSDKName:   ToSnakeCase(paramName),
						Required:    true,
						ReadOnly:    false,
					})
				}
			}
		}

		// Extract Update fields
		if updateSchema, err := g.parser.GetOperationRequestSchema(ops.PartialUpdate); err == nil {
			if fields, err := ExtractFields(updateSchema); err == nil {
				updateFields = fields
			}
		}

		// Extract Response fields (prefer Retrieve operation as it's usually most complete)
		if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		} else if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Create); err == nil {
			// Fallback to Create response
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		}

		// Merge fields for the model (Create + Response)
		allFields := MergeFields(createFields, responseFields)

		// Filter out marketplace and other fields for non-order resources
		if !isOrder {
			// Create a set of input fields to protect them from removal
			inputFields := make(map[string]bool)
			for _, f := range createFields {
				inputFields[f.TFSDKName] = true
			}

			modelFields = make([]FieldInfo, 0)
			for _, f := range allFields {
				// Remove if it's in exclude list AND NOT an input field
				if ExcludedFields[f.TFSDKName] && !inputFields[f.TFSDKName] {
					continue
				}
				modelFields = append(modelFields, f)
			}
		} else {
			modelFields = allFields
		}
	}

	// Enforce Required/Not-ReadOnly for Path Params in ModelFields (for Nested Creation)
	if resource.CreateOperation != nil && len(resource.CreateOperation.PathParams) > 0 {
		pathParams := make(map[string]bool)
		for _, v := range resource.CreateOperation.PathParams {
			pathParams[v] = true
		}

		for i, f := range modelFields {
			if pathParams[f.Name] {
				modelFields[i].Required = true
				modelFields[i].ReadOnly = false
				// Also update createFields to match, so validation passes
				for j, cf := range createFields {
					if cf.Name == f.Name {
						createFields[j].Required = true
						createFields[j].ReadOnly = false
					}
				}
			}
		}
	}

	// Update responseFields to use merged field definitions
	// This ensures shared.tmpl uses the complete schema for nested objects
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

	// Sort all fields for deterministic output
	sort.Slice(createFields, func(i, j int) bool { return createFields[i].Name < createFields[j].Name })
	sort.Slice(updateFields, func(i, j int) bool { return updateFields[i].Name < updateFields[j].Name })
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })
	sort.Slice(modelFields, func(i, j int) bool { return modelFields[i].Name < modelFields[j].Name })
	sort.Slice(updateActions, func(i, j int) bool { return updateActions[i].Name < updateActions[j].Name })

	data := map[string]interface{}{
		"Name":                  resource.Name,
		"Operations":            ops,
		"APIPaths":              apiPaths,
		"CreateFields":          createFields,
		"UpdateFields":          updateFields,
		"ResponseFields":        responseFields,
		"ModelFields":           modelFields,
		"IsOrder":               isOrder,
		"IsLink":                resource.LinkOp != "", // Check if it's a link plugin
		"Source":                resource.Source,
		"Target":                resource.Target,
		"LinkCheckKey":          resource.LinkCheckKey,
		"OfferingType":          resource.OfferingType,
		"UpdateActions":         updateActions, // Use enriched UpdateAction slice with resolved paths
		"TerminationAttributes": resource.TerminationAttributes,
		"CreateOperation":       resource.CreateOperation, // Custom create operation config
		"CompositeKeys":         resource.CompositeKeys,   // Fields forming composite key
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
