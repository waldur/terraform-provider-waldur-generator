package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateSharedUtils generates the utils.go file in internal/sdk/common
func (g *Generator) generateSharedUtils() error {
	// Read template file
	content, err := templates.ReadFile("templates/utils.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read utils template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "sdk", "common", "utils.go")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for utils.go: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Replace package name
	contentStr := strings.Replace(string(content), "package resources", "package common", 1)

	if _, err := f.WriteString(contentStr); err != nil {
		return err
	}

	return nil
}
