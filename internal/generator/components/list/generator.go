package list

import (
	"path/filepath"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// GenerateImplementation generates a list resource file
func GenerateImplementation(cfg *config.Config, renderer common.Renderer, rd *common.ResourceData) error {
	// data for template - list resource template expects some specific flags
	data := ListResourceData{
		Name:              rd.Name,
		Service:           rd.Service,
		CleanName:         rd.CleanName,
		APIPaths:          rd.APIPaths,
		ResponseFields:    rd.ResponseFields,
		ModelFields:       rd.ModelFields,
		FilterParams:      rd.FilterParams,
		ProviderName:      cfg.Generator.ProviderName,
		SkipFilterMapping: true,
	}

	return renderer.RenderTemplate(
		"list_resource.go.tmpl",
		[]string{"templates/shared/*.tmpl", "components/list/list_resource.go.tmpl"},
		data,
		filepath.Join(cfg.Generator.OutputDir, "services", rd.Service, rd.CleanName),
		"list.go",
	)
}
