package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// generateClient creates the API client file
func (g *Generator) generateClient() error {
	tmpl, err := template.ParseFS(templates, "templates/client.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse client template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "client", "client.go")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, nil); err != nil {
		return err
	}

	// Also generate client tests
	return g.generateClientTests()
}

// generateClientTests creates the client_test.go file
func (g *Generator) generateClientTests() error {
	tmpl, err := template.ParseFS(templates, "templates/client_test.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse client test template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "client", "client_test.go")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// Client test template doesn't need any data
	return tmpl.Execute(f, nil)
}
