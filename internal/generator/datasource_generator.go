package generator

import (
	"path/filepath"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

func cloneFields(fields []FieldInfo) []FieldInfo {
	cloned := make([]FieldInfo, len(fields))
	for i, f := range fields {
		cloned[i] = f.Clone()
	}
	return cloned
}

func cloneFilterParams(params []FilterParam) []FilterParam {
	cloned := make([]FilterParam, len(params))
	for i, p := range params {
		cloned[i] = p.Clone()
	}
	return cloned
}

func setIsDataSourceRecursive(fields []FieldInfo) {
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

// generateDataSourceImplementation generates a data source file
func (g *Generator) generateDataSourceImplementation(rd *ResourceData, dataSource *config.DataSource) error {
	// For datasources, all fields must be IsDataSource = true
	// We clone fields to avoid modifying the originals which are shared with Resources.
	responseFields := cloneFields(rd.ResponseFields)
	setIsDataSourceRecursive(responseFields)

	filterParams := cloneFilterParams(rd.FilterParams)
	// FilterParams dont need setIsDataSourceRecursive as they are simple structs

	modelFields := cloneFields(rd.ModelFields)
	setIsDataSourceRecursive(modelFields)

	data := common.DataSourceTemplateData{
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

	return g.renderTemplate(
		"datasource.go.tmpl",
		[]string{"templates/shared.tmpl", "templates/datasource.go.tmpl"},
		data,
		filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName),
		"datasource.go",
	)
}
