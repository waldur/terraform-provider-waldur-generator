package action

import (
	"path/filepath"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// GenerateImplementation generates action files for a resource
func GenerateImplementation(cfg *config.Config, renderer common.Renderer, rd *common.ResourceData) error {
	for _, action := range rd.StandaloneActions {
		data := ActionTemplateData{
			ResourceName:    rd.Name,
			Service:         rd.Service,
			CleanName:       rd.CleanName,
			ActionName:      action.Name,
			OperationID:     action.Operation,
			BaseOperationID: rd.BaseOperationID,
			ProviderName:    cfg.Generator.ProviderName,
			Path:            action.Path,
			IdentifierParam: "uuid",
			IdentifierDesc:  "UUID of the resource",
		}

		if err := renderer.RenderTemplate(
			"action.go.tmpl",
			[]string{"templates/shared.tmpl", "components/action/action.go.tmpl"},
			data,
			filepath.Join(cfg.Generator.OutputDir, "services", rd.Service, rd.CleanName),
			action.Name+".go",
		); err != nil {
			return err
		}
	}
	return nil
}
