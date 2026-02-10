package generator

import (
	"path/filepath"
)

// generateSharedUtils generates the utils.go file in internal/sdk/common
func (g *Generator) generateSharedUtils() error {
	// The template already has "package common", so we can render it directly
	return g.RenderTemplate(
		"utils.go.tmpl",
		[]string{"templates/utils.go.tmpl"},
		nil,
		filepath.Join(g.config.Generator.OutputDir, "internal", "sdk", "common"),
		"utils.go",
	)
}
