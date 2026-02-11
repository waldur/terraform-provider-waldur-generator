package generator

import (
	"path/filepath"
)

// generateSharedUtils generates the shared utility files in internal/sdk/common
func (g *Generator) generateSharedUtils() error {
	templates := []struct {
		tmplName string
		fileName string
	}{
		{"modifiers.go.tmpl", "modifiers.go"},
		{"waldur.go.tmpl", "waldur.go"},
		{"filters.go.tmpl", "filters.go"},
		{"population.go.tmpl", "population.go"},
		{"polling.go.tmpl", "polling.go"},
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "internal", "sdk", "common")

	for _, t := range templates {
		err := g.RenderTemplate(
			t.tmplName,
			[]string{filepath.Join("templates", t.tmplName)},
			nil,
			outputDir,
			t.fileName,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
