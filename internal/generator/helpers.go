package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// RenderTemplate handles the common pattern of parsing a template and executing it to a file
func (g *Generator) RenderTemplate(templateName string, templatePaths []string, data interface{}, outputDir, fileName string) error {
	// Parse templates
	tmpl, err := template.New(templateName).Funcs(GetFuncMap()).ParseFS(templates, templatePaths...)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", templateName, err)
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", outputDir, err)
	}

	// Create output file
	outputPath := filepath.Join(outputDir, fileName)
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outputPath, err)
	}
	defer f.Close()

	// Execute template
	if err := tmpl.ExecuteTemplate(f, templateName, data); err != nil {
		return fmt.Errorf("failed to execute template %s: %w", templateName, err)
	}

	return nil
}

// GetSchemaConfig constructs the standard schema configuration from generator config
func (g *Generator) GetSchemaConfig() common.SchemaConfig {
	excludedMap := make(map[string]bool)
	for _, f := range g.config.Generator.ExcludedFields {
		excludedMap[f] = true
	}
	setMap := make(map[string]bool)
	for _, f := range g.config.Generator.SetFields {
		setMap[f] = true
	}
	return common.SchemaConfig{
		ExcludedFields: excludedMap,
		SetFields:      setMap,
	}
}
