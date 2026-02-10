package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// generateListResourceImplementation generates a list resource file
func (g *Generator) generateListResourceImplementation(rd *ResourceData) error {
	// Determine template name and path
	templateName := "list_resource.go.tmpl"

	tmpl, err := template.New(templateName).Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/list_resource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse list resource template: %w", err)
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	outputPath := filepath.Join(outputDir, "list.go")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// data for template - list resource template expects some specific flags
	data := common.ListResourceData{
		Name:              rd.Name,
		Service:           rd.Service,
		CleanName:         rd.CleanName,
		APIPaths:          rd.APIPaths,
		ResponseFields:    rd.ResponseFields,
		ModelFields:       rd.ModelFields,
		FilterParams:      rd.FilterParams,
		ProviderName:      g.config.Generator.ProviderName,
		SkipFilterMapping: true,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
