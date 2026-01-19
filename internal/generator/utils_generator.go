package generator

import (
	"fmt"
	"os"
	"path/filepath"
)

// generateSharedUtils generates the utils.go file in internal/resources
func (g *Generator) generateSharedUtils() error {
	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", "utils.go")

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for utils.go: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Read template file directly since it contains no logic
	content, err := templates.ReadFile("templates/utils.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read utils template: %w", err)
	}

	if _, err := f.Write(content); err != nil {
		return err
	}

	return nil
}
