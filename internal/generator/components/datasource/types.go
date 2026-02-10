package datasource

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// DataSourceTemplateData holds data for generating data source files
type DataSourceTemplateData struct {
	Name           string
	Service        string
	CleanName      string
	Operations     config.OperationSet
	ListPath       string
	RetrievePath   string
	FilterParams   []common.FilterParam
	ResponseFields []common.FieldInfo
	ModelFields    []common.FieldInfo
}
