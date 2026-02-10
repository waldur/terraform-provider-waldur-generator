package list

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// ListResourceData holds data for generating list resource files
type ListResourceData struct {
	Name              string
	Service           string
	CleanName         string
	APIPaths          map[string]string
	ResponseFields    []common.FieldInfo
	ModelFields       []common.FieldInfo
	FilterParams      []common.FilterParam
	ProviderName      string
	SkipFilterMapping bool
}
