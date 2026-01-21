package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// generateSharedUtils generates the utils.go file in internal/resources and internal/datasources
func (g *Generator) generateSharedUtils() error {
	// Read template file
	content, err := templates.ReadFile("templates/utils.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to read utils template: %w", err)
	}

	// Generate for resources package
	resourcesPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", "utils.go")
	if err := g.writeUtilsFile(resourcesPath, content, "resources"); err != nil {
		return err
	}

	// Generate for datasources package
	datasourcesPath := filepath.Join(g.config.Generator.OutputDir, "internal", "datasources", "utils.go")
	if err := g.writeUtilsFile(datasourcesPath, content, "datasources"); err != nil {
		return err
	}

	return nil
}

func (g *Generator) writeUtilsFile(outputPath string, content []byte, packageName string) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for utils.go: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Replace package name if needed
	contentStr := string(content)
	if packageName != "resources" {
		contentStr = strings.Replace(contentStr, "package resources", "package "+packageName, 1)
	}

	if _, err := f.WriteString(contentStr); err != nil {
		return err
	}

	return nil
}
