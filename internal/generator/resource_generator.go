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

// generateResourceImplementation generates a resource file
func (g *Generator) generateResourceImplementation(rd *ResourceData, resource *config.Resource) error {
	tmpl, err := template.New("resource.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/resource.go.tmpl", "templates/resource_standard.tmpl", "templates/resource_order.tmpl", "templates/resource_link.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse resource template: %w", err)
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	outputPath := filepath.Join(outputDir, "resource.go")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, rd); err != nil {
		return err
	}

	return nil
}

// prepareResourceData extracts fields and info for a resource
func (g *Generator) prepareResourceData(resource *config.Resource) (*ResourceData, error) {
	ops := resource.OperationIDs()

	// 1. Choose builder
	var builder ResourceBuilder
	base := BaseBuilder{g: g, resource: resource, ops: ops}
	if resource.Plugin == "order" {
		builder = &OrderBuilder{BaseBuilder: base}
	} else if resource.Plugin == "link" || resource.LinkOp != "" {
		builder = &LinkBuilder{BaseBuilder: base}
	} else {
		builder = &StandardBuilder{BaseBuilder: base}
	}

	// 2. Build Paths and Fields
	apiPaths := builder.GetAPIPaths()

	createFields, err := builder.BuildCreateFields()
	if err != nil {
		return nil, err
	}
	updateFields, err := builder.BuildUpdateFields()
	if err != nil {
		return nil, err
	}
	responseFields, err := builder.BuildResponseFields()
	if err != nil {
		return nil, err
	}

	// 3. Common Enriched Logic (Actions, Filters, etc.)
	// Resolve update action paths from OpenAPI schema
	var updateActions []UpdateAction
	for actionName, actionConfig := range resource.UpdateActions {
		action := UpdateAction{
			Name:       actionName,
			Operation:  actionConfig.Operation,
			Param:      actionConfig.Param,
			CompareKey: actionConfig.CompareKey,
		}
		if action.CompareKey == "" {
			action.CompareKey = action.Param
		}
		if _, actionPath, _, err := base.g.parser.GetOperation(actionConfig.Operation); err == nil {
			action.Path = actionPath
		}
		updateActions = append(updateActions, action)
	}

	// Resolve standalone actions
	var standaloneActions []UpdateAction
	for _, actionName := range resource.Actions {
		operationID := fmt.Sprintf("%s_%s", resource.BaseOperationID, actionName)
		action := UpdateAction{
			Name:      actionName,
			Operation: operationID,
		}
		if _, actionPath, _, err := base.g.parser.GetOperation(operationID); err == nil {
			action.Path = actionPath
		}
		standaloneActions = append(standaloneActions, action)
	}

	// Extract filter parameters
	var filterParams []FieldInfo
	if op, _, _, err := base.g.parser.GetOperation(ops.List); err == nil {
		for _, paramRef := range op.Parameters {
			if paramRef.Value == nil {
				continue
			}
			param := paramRef.Value
			if param.In == "query" {
				paramName := param.Name
				if paramName == "page" || paramName == "page_size" || paramName == "o" || paramName == "field" {
					continue
				}
				if param.Schema != nil && param.Schema.Value != nil {
					typeStr := getSchemaType(param.Schema.Value)
					goType := GetGoType(typeStr)
					if goType == "" || strings.HasPrefix(goType, "types.List") || strings.HasPrefix(goType, "types.Object") {
						continue
					}
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
		sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })
		for i := range filterParams {
			filterParams[i].Description = GetDefaultDescription(filterParams[i].Name, humanize(resource.Name), filterParams[i].Description)
		}
	}

	// 4. Merge Fields for Model
	var modelFields []FieldInfo
	if resource.Plugin == "order" {
		modelFields = MergeOrderFields(createFields, responseFields)
		// Add Plan and Limits fields manually to ModelFields for Order resources
		modelFields = MergeFields(modelFields, []FieldInfo{
			{Name: "plan", Type: "string", Description: "Plan URL", GoType: "types.String", Required: false},
			{Name: "limits", Type: "object", Description: "Resource limits", GoType: "types.Map", ItemType: "number", Required: false},
		})
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
				Name: term.Name, Type: term.Type, Description: "Termination attribute", GoType: goType,
			})
		}
	} else {
		modelFields = MergeFields(createFields, responseFields)
	}

	// 5. Special Overrides (Marketplace Attributes, Path Params)
	if resource.Name == "marketplace_order" {
		for i := range modelFields {
			if modelFields[i].Name == "attributes" {
				modelFields[i].GoType = "types.Map"
				modelFields[i].ItemType = "string"
				modelFields[i].Type = "object"
				modelFields[i].Properties = nil
			}
		}
		for i := range createFields {
			if createFields[i].Name == "attributes" {
				createFields[i].GoType = "types.Map"
				createFields[i].ItemType = "string"
				createFields[i].Type = "object"
				createFields[i].Properties = nil
			}
		}
	}

	if resource.CreateOperation != nil && len(resource.CreateOperation.PathParams) > 0 {
		pathParamSet := make(map[string]bool)
		for _, v := range resource.CreateOperation.PathParams {
			pathParamSet[v] = true
		}
		for i := range modelFields {
			if pathParamSet[modelFields[i].Name] {
				modelFields[i].Required = true
				modelFields[i].ReadOnly = false
				modelFields[i].IsPathParam = true
			}
		}
		// Ensure path params are in createFields as well
		for _, name := range resource.CreateOperation.PathParams {
			found := false
			for _, f := range createFields {
				if f.Name == name {
					found = true
					break
				}
			}
			if !found {
				createFields = append(createFields, FieldInfo{
					Name: name, Type: "string", Description: "Required path parameter", GoType: "types.String", Required: true, IsPathParam: true,
				})
			}
		}
	}

	// 6. Final Polish (ForceNew, Descriptions, Status)
	validUpdateFields := make(map[string]bool)
	for _, f := range updateFields {
		validUpdateFields[f.Name] = true
	}
	for _, action := range updateActions {
		validUpdateFields[action.Param] = true
	}

	FillDescriptions(modelFields, humanize(resource.Name))
	for i := range modelFields {
		if !modelFields[i].ReadOnly && !validUpdateFields[modelFields[i].Name] {
			modelFields[i].ForceNew = true
		}
	}

	calculateSchemaStatusRecursive(modelFields, createFields, responseFields)

	// Update responseFields to use merged field definitions
	modelMap := make(map[string]FieldInfo)
	for _, f := range modelFields {
		modelMap[f.Name] = f
	}
	for i := range responseFields {
		if mergedF, ok := modelMap[responseFields[i].Name]; ok {
			responseFields[i] = mergedF
		}
	}

	sort.Slice(createFields, func(i, j int) bool { return createFields[i].Name < createFields[j].Name })
	sort.Slice(updateFields, func(i, j int) bool { return updateFields[i].Name < updateFields[j].Name })
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })
	sort.Slice(modelFields, func(i, j int) bool { return modelFields[i].Name < modelFields[j].Name })

	service, cleanName := splitResourceName(resource.Name)
	skipPolling := true
	for _, f := range responseFields {
		if f.Name == "state" || f.Name == "status" {
			skipPolling = false
			break
		}
	}

	inputFields := make(map[string]bool)
	for _, f := range createFields {
		inputFields[f.Name] = true
	}
	ApplySchemaSkipRecursive(modelFields, inputFields)
	ApplySchemaSkipRecursive(responseFields, inputFields)

	rd := &ResourceData{
		Name: resource.Name, Service: service, CleanName: cleanName, Plugin: resource.Plugin,
		Operations: ops, APIPaths: apiPaths, CreateFields: createFields, UpdateFields: updateFields,
		ResponseFields: responseFields, ModelFields: modelFields, IsOrder: resource.Plugin == "order",
		IsLink: resource.LinkOp != "", Source: resource.Source, Target: resource.Target,
		LinkCheckKey: resource.LinkCheckKey, OfferingType: resource.OfferingType,
		UpdateActions: updateActions, StandaloneActions: standaloneActions, Actions: resource.Actions,
		TerminationAttributes: resource.TerminationAttributes, CreateOperation: resource.CreateOperation,
		CompositeKeys: resource.CompositeKeys, FilterParams: filterParams, SkipPolling: skipPolling,
		HasDataSource: g.hasDataSource(resource.Name),
	}

	seenHashes := make(map[string]string)
	seenNames := make(map[string]string)
	assignMissingAttrTypeRefs(rd.ModelFields, "", seenHashes, seenNames)
	assignMissingAttrTypeRefs(rd.ResponseFields, "", seenHashes, seenNames)
	rd.NestedStructs = collectUniqueStructs(rd.ModelFields)

	return rd, nil
}

// collectUniqueStructs gathers all Nested structs that have a AttrTypeRef defined
func collectUniqueStructs(params ...[]FieldInfo) []FieldInfo {
	seen := make(map[string]bool)
	var result []FieldInfo
	var traverse func([]FieldInfo)

	traverse = func(fields []FieldInfo) {
		for _, f := range fields {
			// Check object type with AttrTypeRef or RefName
			if f.GoType == "types.Object" {
				key := f.AttrTypeRef
				if key == "" {
					key = f.RefName
				}
				if key != "" {
					if !seen[key] {
						seen[key] = true
						// Ensure AttrTypeRef is set for consistency in result
						if f.AttrTypeRef == "" {
							f.AttrTypeRef = key
						}
						result = append(result, f)
						traverse(f.Properties)
					}
				} else {
					traverse(f.Properties)
				}
			}
			// Check list/set of objects with AttrTypeRef or RefName
			if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemSchema != nil {
				key := f.ItemSchema.AttrTypeRef
				if key == "" {
					key = f.ItemSchema.RefName
				}
				if key != "" {
					if !seen[key] {
						seen[key] = true
						// Ensure AttrTypeRef is set
						if f.ItemSchema.AttrTypeRef == "" {
							f.ItemSchema.AttrTypeRef = key
						}
						result = append(result, *f.ItemSchema)
						traverse(f.ItemSchema.Properties)
					}
				} else {
					traverse(f.ItemSchema.Properties)
				}
			}
		}
	}

	for _, p := range params {
		traverse(p)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].AttrTypeRef < result[j].AttrTypeRef })
	return result
}

// assignMissingAttrTypeRefs recursively assigns a AttrTypeRef to objects/lists of objects that lack one.
// It uses content-based hashing to ensure that identical structures share the same helper function name,
// while different structures get unique names even if they share the same RefName.
func assignMissingAttrTypeRefs(fields []FieldInfo, prefix string, seenHashes map[string]string, seenNames map[string]string) {
	for i := range fields {
		f := &fields[i]

		// Recursively process children first (Bottom-Up) to ensure their AttrTypeRefs are set
		if f.GoType == "types.Object" {
			assignMissingAttrTypeRefs(f.Properties, prefix+toTitle(f.Name), seenHashes, seenNames)
		} else if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemSchema != nil {
			if f.ItemSchema.GoType == "types.Object" {
				assignMissingAttrTypeRefs(f.ItemSchema.Properties, prefix+toTitle(f.Name), seenHashes, seenNames)

				// Also assign ref to ItemSchema itself if it's an object
				hash := computeStructHash(*f.ItemSchema)
				if name, ok := seenHashes[hash]; ok {
					f.ItemSchema.AttrTypeRef = name
				} else {
					candidate := f.ItemSchema.RefName
					if candidate == "" {
						candidate = prefix + toTitle(f.Name)
					}
					finalName := resolveUniqueName(candidate, hash, seenNames)
					seenHashes[hash] = finalName
					seenNames[finalName] = hash
					f.ItemSchema.AttrTypeRef = finalName
				}
			}
		}

		// Now process f itself if it is Object
		if f.GoType == "types.Object" {
			hash := computeStructHash(*f)
			if name, ok := seenHashes[hash]; ok {
				f.AttrTypeRef = name
			} else {
				candidate := f.RefName
				if candidate == "" {
					candidate = prefix + toTitle(f.Name)
				}
				finalName := resolveUniqueName(candidate, hash, seenNames)
				seenHashes[hash] = finalName
				seenNames[finalName] = hash
				f.AttrTypeRef = finalName
			}
		}
	}
}

func resolveUniqueName(candidate string, hash string, seenNames map[string]string) string {
	finalName := candidate
	counter := 2
	for {
		if oldHash, exists := seenNames[finalName]; exists {
			if oldHash == hash {
				return finalName // Same content, same name
			}
			// Collision: same name but different content
			finalName = fmt.Sprintf("%s%d", candidate, counter)
			counter++
		} else {
			return finalName
		}
	}
}

func computeStructHash(f FieldInfo) string {
	var parts []string
	for _, p := range f.Properties {
		// key: Name + Type + AttrTypeRef (for deep equality)
		// We use AttrTypeRef because child structs already have it assigned
		key := fmt.Sprintf("%s:%s:%s", p.Name, p.GoType, p.AttrTypeRef)
		parts = append(parts, key)
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

// generateModel creates the shared model file for a resource
func (g *Generator) generateModel(res *ResourceData) error {
	tmpl, err := template.New("model.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/model.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse model template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "services", res.Service, res.CleanName, "model.go")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, res)
}

// calculateSchemaStatusRecursive recursively determines ServerComputed, UseStateForUnknown,
// and adjusts Required status for nested fields.
func calculateSchemaStatusRecursive(fields []FieldInfo, createFields, responseFields []FieldInfo) {
	createMap := make(map[string]FieldInfo)
	for _, f := range createFields {
		createMap[f.Name] = f
	}

	responseMap := make(map[string]FieldInfo)
	for _, f := range responseFields {
		responseMap[f.Name] = f
	}

	for i := range fields {
		f := &fields[i]

		// ServerComputed logic
		cf, inCreate := createMap[f.Name]
		_, inResponse := responseMap[f.Name]

		if f.ReadOnly {
			f.ServerComputed = false
		} else if !inCreate {
			f.ServerComputed = true
		} else if !cf.Required && inResponse {
			f.ServerComputed = true
		}

		// UseStateForUnknown logic
		if f.ServerComputed || f.ReadOnly {
			f.UseStateForUnknown = true
		}

		// If it's ServerComputed, it shouldn't be Required in Terraform
		// as it might be populated by the server instead of the user.
		if f.ServerComputed && f.Required {
			f.Required = false
		}

		// Recursively process nested types
		if f.GoType == "types.Object" {
			var subCreate, subResponse []FieldInfo
			if inCreate {
				subCreate = cf.Properties
			}
			if inResponse {
				subResponse = responseMap[f.Name].Properties
			}
			calculateSchemaStatusRecursive(f.Properties, subCreate, subResponse)
		} else if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemSchema != nil {
			var subCreate, subResponse []FieldInfo
			if inCreate && cf.ItemSchema != nil {
				subCreate = cf.ItemSchema.Properties
			}
			if inResponse && responseMap[f.Name].ItemSchema != nil {
				subResponse = responseMap[f.Name].ItemSchema.Properties
			}
			calculateSchemaStatusRecursive(f.ItemSchema.Properties, subCreate, subResponse)
		}
	}
}

// ResourceBuilder defines the interface for building resource-specific data
type ResourceBuilder interface {
	BuildCreateFields() ([]FieldInfo, error)
	BuildUpdateFields() ([]FieldInfo, error)
	BuildResponseFields() ([]FieldInfo, error)
	GetAPIPaths() map[string]string
}

// BaseBuilder provides common functionality for all builders
type BaseBuilder struct {
	g        *Generator
	resource *config.Resource
	ops      config.OperationSet
}

func (b *BaseBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	// Get path from list operation (used as base path)
	if _, listPath, _, err := b.g.parser.GetOperation(b.ops.List); err == nil {
		paths["Base"] = listPath
	}

	// Get path from create operation
	createOp := b.ops.Create
	if b.resource.CreateOperation != nil && b.resource.CreateOperation.OperationID != "" {
		createOp = b.resource.CreateOperation.OperationID
		if _, createPath, _, err := b.g.parser.GetOperation(createOp); err == nil {
			paths["Create"] = createPath
			paths["CreateOperationID"] = createOp
			for k, v := range b.resource.CreateOperation.PathParams {
				paths["CreatePathParam_"+k] = v
			}
		}
	} else if _, createPath, _, err := b.g.parser.GetOperation(createOp); err == nil {
		paths["Create"] = createPath
	}

	// Get path from retrieve operation
	if _, retrievePath, _, err := b.g.parser.GetOperation(b.ops.Retrieve); err == nil {
		paths["Retrieve"] = retrievePath
	}

	// Get path from update operation
	if _, updatePath, _, err := b.g.parser.GetOperation(b.ops.PartialUpdate); err == nil {
		paths["Update"] = updatePath
	}

	// Get path from delete operation
	if _, deletePath, _, err := b.g.parser.GetOperation(b.ops.Destroy); err == nil {
		paths["Delete"] = deletePath
	}

	return paths
}

// StandardBuilder implements ResourceBuilder for standard resources
type StandardBuilder struct {
	BaseBuilder
}

func (b *StandardBuilder) BuildCreateFields() ([]FieldInfo, error) {
	createOp := b.ops.Create
	if b.resource.CreateOperation != nil && b.resource.CreateOperation.OperationID != "" {
		createOp = b.resource.CreateOperation.OperationID
	}
	schema, err := b.g.parser.GetOperationRequestSchema(createOp)
	if err != nil {
		return nil, nil // Some resources might not have a create schema
	}
	return ExtractFields(schema, true)
}

func (b *StandardBuilder) BuildUpdateFields() ([]FieldInfo, error) {
	schema, err := b.g.parser.GetOperationRequestSchema(b.ops.PartialUpdate)
	if err != nil {
		return nil, nil
	}
	return ExtractFields(schema, true)
}

func (b *StandardBuilder) BuildResponseFields() ([]FieldInfo, error) {
	if schema, err := b.g.parser.GetOperationResponseSchema(b.ops.Retrieve); err == nil {
		return ExtractFields(schema, true)
	}
	if schema, err := b.g.parser.GetOperationResponseSchema(b.ops.Create); err == nil {
		return ExtractFields(schema, true)
	}
	return nil, nil
}

// OrderBuilder implements ResourceBuilder for marketplace order resources
type OrderBuilder struct {
	BaseBuilder
}

func (b *OrderBuilder) BuildCreateFields() ([]FieldInfo, error) {
	schemaName := strings.ReplaceAll(b.resource.OfferingType, ".", "") + "CreateOrderAttributes"
	offeringSchema, err := b.g.parser.GetSchema(schemaName)
	if err != nil {
		return nil, fmt.Errorf("failed to find offering schema %s: %w", schemaName, err)
	}
	fields, err := ExtractFields(offeringSchema, true)
	if err != nil {
		return nil, err
	}
	for i := range fields {
		fields[i].Required = false
	}
	// Add required offering and project fields
	fields = append(fields, FieldInfo{
		Name: "offering", Type: "string", Description: "Offering URL", GoType: "types.String", Required: true,
	}, FieldInfo{
		Name: "project", Type: "string", Description: "Project URL", GoType: "types.String", Required: true,
	})
	return fields, nil
}

func (b *OrderBuilder) BuildUpdateFields() ([]FieldInfo, error) {
	schema, err := b.g.parser.GetOperationRequestSchema(b.ops.PartialUpdate)
	if err != nil {
		return nil, nil
	}
	return ExtractFields(schema, true)
}

func (b *OrderBuilder) BuildResponseFields() ([]FieldInfo, error) {
	schema, err := b.g.parser.GetOperationResponseSchema(b.ops.Retrieve)
	if err != nil {
		return nil, err
	}
	return ExtractFields(schema, true)
}

func (b *OrderBuilder) GetAPIPaths() map[string]string {
	paths := b.BaseBuilder.GetAPIPaths()
	return paths
}

// LinkBuilder implements ResourceBuilder for link resources
type LinkBuilder struct {
	BaseBuilder
}

func (b *LinkBuilder) BuildCreateFields() ([]FieldInfo, error) {
	schema, err := b.g.parser.GetOperationRequestSchema(b.resource.LinkOp)
	if err != nil {
		return nil, nil
	}
	fields, err := ExtractFields(schema, true)
	if err != nil {
		return nil, err
	}
	// Source and Target handling
	if b.resource.Source != nil && b.resource.Source.Param != "" {
		found := false
		for _, f := range fields {
			if f.Name == b.resource.Source.Param {
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, FieldInfo{
				Name: b.resource.Source.Param, Type: "string", Description: "Source resource UUID", GoType: "types.String", Required: true,
			})
		}
	}
	if b.resource.Target != nil && b.resource.Target.Param != "" {
		found := false
		for _, f := range fields {
			if f.Name == b.resource.Target.Param {
				found = true
				break
			}
		}
		if !found {
			fields = append(fields, FieldInfo{
				Name: b.resource.Target.Param, Type: "string", Description: "Target resource UUID", GoType: "types.String", Required: true,
			})
		}
	}
	// LinkParams
	for _, param := range b.resource.LinkParams {
		found := false
		for _, f := range fields {
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
			fields = append(fields, FieldInfo{
				Name: param.Name, Type: param.Type, Description: "Link parameter", GoType: goType, Required: false,
			})
		}
	}
	return fields, nil
}

func (b *LinkBuilder) BuildUpdateFields() ([]FieldInfo, error) {
	return nil, nil
}

func (b *LinkBuilder) BuildResponseFields() ([]FieldInfo, error) {
	if schema, err := b.g.parser.GetOperationResponseSchema(b.ops.Retrieve); err == nil {
		return ExtractFields(schema, true)
	}
	return nil, nil
}

func (b *LinkBuilder) GetAPIPaths() map[string]string {
	paths := make(map[string]string)
	if _, listPath, _, err := b.g.parser.GetOperation(b.ops.List); err == nil {
		paths["Base"] = listPath
	}
	if _, retrievePath, _, err := b.g.parser.GetOperation(b.ops.Retrieve); err == nil {
		paths["Retrieve"] = retrievePath
	}
	if _, linkPath, _, err := b.g.parser.GetOperation(b.resource.LinkOp); err == nil {
		paths["Link"] = linkPath
	}
	if _, unlinkPath, _, err := b.g.parser.GetOperation(b.resource.UnlinkOp); err == nil {
		paths["Unlink"] = unlinkPath
	}
	if b.resource.Source != nil && b.resource.Source.RetrieveOp != "" {
		if _, sourcePath, _, err := b.g.parser.GetOperation(b.resource.Source.RetrieveOp); err == nil {
			paths["SourceRetrieve"] = sourcePath
		}
	}
	return paths
}
