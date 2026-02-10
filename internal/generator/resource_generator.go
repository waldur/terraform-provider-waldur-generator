package generator

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/builders"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// generateResourceImplementation generates a resource file
func (g *Generator) generateResourceImplementation(rd *ResourceData) error {
	return g.renderTemplate(
		"resource.go.tmpl",
		rd.TemplateFiles,
		rd,
		filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName),
		"resource.go",
	)
}

// prepareResourceData extracts fields and info for a resource
func (g *Generator) prepareResourceData(resource *config.Resource) (*ResourceData, error) {
	ops := resource.OperationIDs()

	// 0. Construct SchemaConfig
	cfg := g.getSchemaConfig()

	// 1. Choose builder
	var builder builders.ResourceBuilder
	base := builders.BaseBuilder{Parser: g.parser, Resource: resource, Ops: ops, SchemaConfig: cfg}
	if resource.Plugin == "order" {
		builder = &builders.OrderBuilder{BaseBuilder: base}
	} else if resource.Plugin == "link" || resource.LinkOp != "" {
		builder = &builders.LinkBuilder{BaseBuilder: base}
	} else {
		builder = &builders.StandardBuilder{BaseBuilder: base}
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
	var updateActions []common.UpdateAction
	actionNames := make([]string, 0, len(resource.UpdateActions))
	for name := range resource.UpdateActions {
		actionNames = append(actionNames, name)
	}
	sort.Strings(actionNames)

	for _, actionName := range actionNames {
		actionConfig := resource.UpdateActions[actionName]
		action := common.UpdateAction{
			Name:       actionName,
			Operation:  actionConfig.Operation,
			Param:      actionConfig.Param,
			CompareKey: actionConfig.CompareKey,
		}
		if action.CompareKey == "" {
			action.CompareKey = action.Param
		}
		if _, actionPath, _, err := g.parser.GetOperation(actionConfig.Operation); err == nil {
			action.Path = actionPath
		}
		updateActions = append(updateActions, action)
	}

	// Resolve standalone actions
	var standaloneActions []common.UpdateAction
	for _, actionName := range resource.Actions {
		operationID := fmt.Sprintf("%s_%s", resource.BaseOperationID, actionName)
		action := common.UpdateAction{
			Name:      actionName,
			Operation: operationID,
		}
		if _, actionPath, _, err := g.parser.GetOperation(operationID); err == nil {
			action.Path = actionPath
		}
		standaloneActions = append(standaloneActions, action)
	}

	// Extract filter parameters
	var filterParams []common.FilterParam
	if op, _, _, err := g.parser.GetOperation(ops.List); err == nil {
		filterParams = common.ExtractFilterParams(op, humanize(resource.Name))
	}

	// 4. Merge Fields for Model
	modelFields, err := builder.BuildModelFields(createFields, responseFields)
	if err != nil {
		return nil, err
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

	common.FillDescriptions(modelFields, humanize(resource.Name))
	for i := range modelFields {
		if !modelFields[i].ReadOnly && !validUpdateFields[modelFields[i].Name] {
			modelFields[i].ForceNew = true
		}
	}

	common.CalculateSchemaStatusRecursive(modelFields, createFields, responseFields)

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

	service, cleanName := common.SplitResourceName(resource.Name)
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
	common.ApplySchemaSkipRecursive(cfg, modelFields, inputFields)
	common.ApplySchemaSkipRecursive(cfg, responseFields, inputFields)

	rd := &ResourceData{
		Name: resource.Name, Service: service, CleanName: cleanName, Plugin: resource.Plugin,
		Operations: ops, APIPaths: apiPaths, CreateFields: createFields, UpdateFields: updateFields,
		ResponseFields: responseFields, ModelFields: modelFields, IsOrder: resource.Plugin == "order",
		IsLink: resource.LinkOp != "", Source: resource.Source, Target: resource.Target,
		LinkCheckKey: resource.LinkCheckKey, OfferingType: resource.OfferingType,
		UpdateActions: updateActions, StandaloneActions: standaloneActions,
		TerminationAttributes: resource.TerminationAttributes, CreateOperation: resource.CreateOperation,
		CompositeKeys: resource.CompositeKeys, FilterParams: filterParams, SkipPolling: skipPolling,
		BaseOperationID: resource.BaseOperationID,
		HasDataSource:   g.hasDataSource(resource.Name),
	}

	seenHashes := make(map[string]string)
	seenNames := make(map[string]string)
	common.AssignMissingAttrTypeRefs(cfg, rd.ModelFields, "", seenHashes, seenNames)
	common.AssignMissingAttrTypeRefs(cfg, rd.ResponseFields, "", seenHashes, seenNames)
	rd.NestedStructs = common.CollectUniqueStructs(rd.ModelFields)
	rd.TemplateFiles = builder.GetTemplateFiles()

	return rd, nil
}

// generateModel creates the shared model file for a resource
func (g *Generator) generateModel(res *ResourceData) error {
	return g.renderTemplate(
		"model.go.tmpl",
		[]string{"templates/shared.tmpl", "templates/model.go.tmpl"},
		res,
		filepath.Join(g.config.Generator.OutputDir, "services", res.Service, res.CleanName),
		"model.go",
	)
}
