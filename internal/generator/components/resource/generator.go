package resource

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins/link"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins/order"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/plugins/standard"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

// GenerateImplementation generates a resource file
func GenerateImplementation(cfg *config.Config, renderer common.Renderer, rd *common.ResourceData) error {
	return renderer.RenderTemplate(
		"resource.go.tmpl",
		rd.TemplateFiles,
		rd,
		filepath.Join(cfg.Generator.OutputDir, "services", rd.Service, rd.CleanName),
		"resource.go",
	)
}

// PrepareData extracts fields and info for a resource
func PrepareData(cfg *config.Config, parser *openapi.Parser, resource *config.Resource, hasDataSource func(string) bool, getSchemaConfig func() common.SchemaConfig) (*common.ResourceData, error) {
	ops := resource.OperationIDs()

	// 0. Construct SchemaConfig
	schemaCfg := getSchemaConfig()

	// 1. Choose builder
	var builder plugins.ResourceBuilder
	base := plugins.BaseBuilder{Parser: parser, Resource: resource, Ops: ops, SchemaConfig: schemaCfg}
	if resource.Plugin == "order" {
		builder = &order.OrderBuilder{BaseBuilder: base}
	} else if resource.Plugin == "link" || resource.LinkOp != "" {
		builder = &link.LinkBuilder{BaseBuilder: base}
	} else {
		builder = &standard.StandardBuilder{BaseBuilder: base}
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
		if _, actionPath, _, err := parser.GetOperation(actionConfig.Operation); err == nil {
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
		if _, actionPath, _, err := parser.GetOperation(operationID); err == nil {
			action.Path = actionPath
		}
		standaloneActions = append(standaloneActions, action)
	}

	// Extract filter parameters
	var filterParams []common.FilterParam
	if op, _, _, err := parser.GetOperation(ops.List); err == nil {
		filterParams = common.ExtractFilterParams(op, common.Humanize(resource.Name))
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
				createFields = append(createFields, common.FieldInfo{
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

	common.FillDescriptions(modelFields, common.Humanize(resource.Name))
	for i := range modelFields {
		if !modelFields[i].ReadOnly && !validUpdateFields[modelFields[i].Name] {
			modelFields[i].ForceNew = true
		}
	}

	common.CalculateSchemaStatusRecursive(modelFields, createFields, responseFields)

	// Update responseFields to use merged field definitions
	modelMap := make(map[string]common.FieldInfo)
	for _, f := range modelFields {
		modelMap[f.Name] = f
	}
	for i := range responseFields {
		if mergedF, ok := modelMap[responseFields[i].Name]; ok {
			responseFields[i] = mergedF
		}
	}

	// Define a generic sorter
	sortByName := func(a, b common.FieldInfo) int {
		return strings.Compare(a.Name, b.Name)
	}

	slices.SortFunc(createFields, sortByName)
	slices.SortFunc(updateFields, sortByName)
	slices.SortFunc(responseFields, sortByName)
	slices.SortFunc(modelFields, sortByName)

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
	common.ApplySchemaSkipRecursive(schemaCfg, modelFields, inputFields)
	common.ApplySchemaSkipRecursive(schemaCfg, responseFields, inputFields)

	rd := &common.ResourceData{
		Name:                  resource.Name,
		Service:               service,
		CleanName:             cleanName,
		Plugin:                resource.Plugin,
		Operations:            ops,
		APIPaths:              apiPaths,
		CreateFields:          createFields,
		UpdateFields:          updateFields,
		ResponseFields:        responseFields,
		ModelFields:           modelFields,
		IsOrder:               resource.Plugin == "order",
		Source:                resource.Source,
		Target:                resource.Target,
		LinkCheckKey:          resource.LinkCheckKey,
		OfferingType:          resource.OfferingType,
		UpdateActions:         updateActions,
		StandaloneActions:     standaloneActions,
		TerminationAttributes: resource.TerminationAttributes,
		CreateOperation:       resource.CreateOperation,
		CompositeKeys:         resource.CompositeKeys,
		FilterParams:          filterParams,
		SkipPolling:           skipPolling,
		BaseOperationID:       resource.BaseOperationID,
		HasDataSource:         hasDataSource(resource.Name),
	}

	seenHashes := make(map[string]string)
	seenNames := make(map[string]string)
	common.AssignMissingAttrTypeRefs(schemaCfg, rd.ModelFields, "", seenHashes, seenNames)
	common.AssignMissingAttrTypeRefs(schemaCfg, rd.ResponseFields, "", seenHashes, seenNames)
	rd.NestedStructs = common.CollectUniqueStructs(rd.ModelFields)
	rd.TemplateFiles = builder.GetTemplateFiles()

	return rd, nil
}

// GenerateModel creates the shared model file for a resource
func GenerateModel(cfg *config.Config, renderer common.Renderer, res *common.ResourceData) error {
	return renderer.RenderTemplate(
		"model.go.tmpl",
		[]string{"templates/shared.tmpl", "components/resource/model.go.tmpl"},
		res,
		filepath.Join(cfg.Generator.OutputDir, "services", res.Service, res.CleanName),
		"model.go",
	)
}
