package datasource

import (
	"path/filepath"
	"sort"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

func cloneFields(fields []common.FieldInfo) []common.FieldInfo {
	cloned := make([]common.FieldInfo, len(fields))
	for i, f := range fields {
		cloned[i] = f.Clone()
	}
	return cloned
}

func cloneFilterParams(params []common.FilterParam) []common.FilterParam {
	cloned := make([]common.FilterParam, len(params))
	for i, p := range params {
		cloned[i] = p.Clone()
	}
	return cloned
}

func setIsDataSourceRecursive(fields []common.FieldInfo) {
	for i := range fields {
		fields[i].IsDataSource = true
		fields[i].Required = false
		fields[i].ReadOnly = true
		if fields[i].ItemSchema != nil {
			fields[i].ItemSchema.IsDataSource = true
			fields[i].ItemSchema.Required = false
			fields[i].ItemSchema.ReadOnly = true
			if len(fields[i].ItemSchema.Properties) > 0 {
				setIsDataSourceRecursive(fields[i].ItemSchema.Properties)
			}
		}
		if len(fields[i].Properties) > 0 {
			setIsDataSourceRecursive(fields[i].Properties)
		}
	}
}

// GenerateImplementation generates a data source file
func GenerateImplementation(cfg *config.Config, renderer common.Renderer, rd *common.ResourceData, dataSource *config.DataSource) error {
	// For datasources, all fields must be IsDataSource = true
	// We clone fields to avoid modifying the originals which are shared with Resources.
	responseFields := cloneFields(rd.ResponseFields)
	setIsDataSourceRecursive(responseFields)

	filterParams := cloneFilterParams(rd.FilterParams)
	// FilterParams dont need setIsDataSourceRecursive as they are simple structs

	modelFields := cloneFields(rd.ModelFields)
	setIsDataSourceRecursive(modelFields)

	data := DataSourceTemplateData{
		Name:           rd.Name,
		Service:        rd.Service,
		CleanName:      rd.CleanName,
		Operations:     rd.Operations,
		ListPath:       rd.APIPaths["Base"],
		RetrievePath:   rd.APIPaths["Retrieve"],
		FilterParams:   filterParams,
		ResponseFields: responseFields,
		ModelFields:    modelFields,
	}

	return renderer.RenderTemplate(
		"datasource.go.tmpl",
		[]string{"templates/shared.tmpl", "components/datasource/datasource.go.tmpl"},
		data,
		filepath.Join(cfg.Generator.OutputDir, "services", rd.Service, rd.CleanName),
		"datasource.go",
	)
}

// PrepareData creates minimal ResourceData for a datasource-only definition
func PrepareData(parser *openapi.Parser, dataSource *config.DataSource, schemaCfg common.SchemaConfig) (*common.ResourceData, error) {
	ops := dataSource.OperationIDs()

	// Extract API paths from OpenAPI operations
	listPath := ""
	retrievePath := ""

	if _, path, _, err := parser.GetOperation(ops.List); err == nil {
		listPath = path
	}

	if _, retPath, _, err := parser.GetOperation(ops.Retrieve); err == nil {
		retrievePath = retPath
	}

	// Extract Response fields
	var responseFields []common.FieldInfo
	if responseSchema, err := parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
		if fields, err := common.ExtractFields(schemaCfg, responseSchema, true); err == nil {
			responseFields = fields
		}
	} else if responseSchema, err := parser.GetOperationResponseSchema(ops.List); err == nil {
		if responseSchema.Value.Type != nil && (*responseSchema.Value.Type)[0] == "array" && responseSchema.Value.Items != nil {
			if fields, err := common.ExtractFields(schemaCfg, responseSchema.Value.Items, true); err == nil {
				responseFields = fields
			}
		}
	}

	// Extract filter parameters
	var filterParams []common.FilterParam
	if op, _, _, err := parser.GetOperation(ops.List); err == nil {
		filterParams = common.ExtractFilterParams(op, common.Humanize(dataSource.Name))
	}

	// Use response fields for model
	modelFields := make([]common.FieldInfo, len(responseFields))
	for i, f := range responseFields {
		modelFields[i] = f.Clone()
	}

	// Filter out marketplace and other fields from schema recursively
	common.ApplySchemaSkipRecursive(schemaCfg, modelFields, nil)
	common.ApplySchemaSkipRecursive(schemaCfg, responseFields, nil)

	// Sort for deterministic output
	sort.Slice(responseFields, func(i, j int) bool { return responseFields[i].Name < responseFields[j].Name })
	sort.Slice(modelFields, func(i, j int) bool { return modelFields[i].Name < modelFields[j].Name })

	// Split name into service and clean name
	service, cleanName := common.SplitResourceName(dataSource.Name)

	return &common.ResourceData{
		Name:             dataSource.Name,
		Service:          service,
		CleanName:        cleanName,
		ResponseFields:   responseFields,
		ModelFields:      modelFields,
		IsDatasourceOnly: true,
		HasDataSource:    true,
		FilterParams:     filterParams,
		APIPaths: map[string]string{
			"Base":     listPath,
			"Retrieve": retrievePath,
		},
		Operations: ops,
	}, nil
}
