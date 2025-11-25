package generator

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/waldur/terraform-waldur-provider-generator/internal/config"
	"github.com/waldur/terraform-waldur-provider-generator/internal/openapi"
)

//go:embed templates/*
var templates embed.FS

// toTitle converts a string to title case for use in templates
func toTitle(s string) string {
	// Convert snake_case to TitleCase
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// Generator orchestrates the provider code generation
type Generator struct {
	config *config.Config
	parser *openapi.Parser
}

// New creates a new generator instance
func New(cfg *config.Config, parser *openapi.Parser) *Generator {
	return &Generator{
		config: cfg,
		parser: parser,
	}
}

// Generate creates the Terraform provider code
func (g *Generator) Generate() error {
	// Validate all operation IDs exist in OpenAPI schema
	if err := g.validateOperations(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Create output directory structure
	if err := g.createDirectoryStructure(); err != nil {
		return fmt.Errorf("failed to create directory structure: %w", err)
	}

	// Generate provider files
	if err := g.generateProvider(); err != nil {
		return fmt.Errorf("failed to generate provider: %w", err)
	}

	// Generate resources
	for _, resource := range g.config.Resources {
		if err := g.generateResource(&resource); err != nil {
			return fmt.Errorf("failed to generate resource %s: %w", resource.Name, err)
		}
	}

	// Generate data sources
	for _, dataSource := range g.config.DataSources {
		if err := g.generateDataSource(&dataSource); err != nil {
			return fmt.Errorf("failed to generate data source %s: %w", dataSource.Name, err)
		}
	}

	// Generate supporting files
	if err := g.generateSupportingFiles(); err != nil {
		return fmt.Errorf("failed to generate supporting files: %w", err)
	}

	return nil
}

// validateOperations checks that all referenced operations exist in the OpenAPI schema
func (g *Generator) validateOperations() error {
	for _, resource := range g.config.Resources {
		ops := resource.OperationIDs()
		for _, opID := range []string{ops.List, ops.Create, ops.Retrieve, ops.PartialUpdate, ops.Destroy} {
			if err := g.parser.ValidateOperationExists(opID); err != nil {
				return fmt.Errorf("resource %s: %w", resource.Name, err)
			}
		}
	}

	for _, dataSource := range g.config.DataSources {
		ops := dataSource.OperationIDs()
		if err := g.parser.ValidateOperationExists(ops.List); err != nil {
			return fmt.Errorf("data source %s: %w", dataSource.Name, err)
		}
	}

	return nil
}

// createDirectoryStructure creates the output directory structure
func (g *Generator) createDirectoryStructure() error {
	dirs := []string{
		g.config.Generator.OutputDir,
		filepath.Join(g.config.Generator.OutputDir, "internal", "provider"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "resources"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "datasources"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "client"),
		filepath.Join(g.config.Generator.OutputDir, "examples"),
		filepath.Join(g.config.Generator.OutputDir, ".github", "workflows"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// generateProvider generates the main provider file
func (g *Generator) generateProvider() error {
	tmpl, err := template.New("provider.go.tmpl").Funcs(template.FuncMap{
		"title": toTitle,
	}).ParseFS(templates, "templates/provider.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse provider template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "provider", "provider.go")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
		"Resources":    g.config.Resources,
		"DataSources":  g.config.DataSources,
	}

	return tmpl.Execute(f, data)
}

// generateResource generates a resource file
func (g *Generator) generateResource(resource *config.Resource) error {
	tmpl, err := template.New("resource.go.tmpl").Funcs(template.FuncMap{
		"title": toTitle,
	}).ParseFS(templates, "templates/resource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse resource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", fmt.Sprintf("%s.go", resource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := map[string]interface{}{
		"Name":       resource.Name,
		"Operations": resource.OperationIDs(),
	}

	return tmpl.Execute(f, data)
}

// generateDataSource generates a data source file
func (g *Generator) generateDataSource(dataSource *config.DataSource) error {
	tmpl, err := template.New("datasource.go.tmpl").Funcs(template.FuncMap{
		"title": toTitle,
	}).ParseFS(templates, "templates/datasource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse datasource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "datasources", fmt.Sprintf("%s.go", dataSource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := map[string]interface{}{
		"Name":       dataSource.Name,
		"Operations": dataSource.OperationIDs(),
	}

	return tmpl.Execute(f, data)
}

// generateSupportingFiles generates go.mod, README, etc.
func (g *Generator) generateSupportingFiles() error {
	// Generate client
	if err := g.generateClient(); err != nil {
		return err
	}

	// Generate main.go
	if err := g.generateMain(); err != nil {
		return err
	}

	// Generate go.mod
	if err := g.generateGoMod(); err != nil {
		return err
	}

	// Generate .goreleaser.yml
	if err := g.generateGoReleaser(); err != nil {
		return err
	}

	// Generate terraform-registry-manifest.json
	if err := g.generateRegistryManifest(); err != nil {
		return err
	}

	// Generate GitHub Actions workflow
	if err := g.generateGitHubWorkflow(); err != nil {
		return err
	}

	return nil
}

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

	// Client template doesn't need any data
	return tmpl.Execute(f, nil)
}

// generateMain creates the main.go file for the generated provider
func (g *Generator) generateMain() error {
	tmpl, err := template.New("main.go.tmpl").ParseFS(templates, "templates/main.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse main template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "main.go")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
	}

	return tmpl.Execute(f, data)
}

// generateGoMod creates the go.mod file for the generated provider
func (g *Generator) generateGoMod() error {
	content := fmt.Sprintf(`module github.com/waldur/terraform-%s-provider

go 1.24

require (
	github.com/hashicorp/terraform-plugin-framework v1.15.0
	github.com/hashicorp/terraform-plugin-go v0.25.0
)
`, g.config.Generator.ProviderName)

	path := filepath.Join(g.config.Generator.OutputDir, "go.mod")
	return os.WriteFile(path, []byte(content), 0644)
}

// generateGoReleaser creates the .goreleaser.yml file
func (g *Generator) generateGoReleaser() error {
	tmpl, err := template.ParseFS(templates, "templates/goreleaser.yml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse goreleaser template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, ".goreleaser.yml")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
	}

	return tmpl.Execute(f, data)
}

// generateRegistryManifest creates the terraform-registry-manifest.json file
func (g *Generator) generateRegistryManifest() error {
	content := `{
  "version": 1,
  "metadata": {
    "protocol_versions": ["6.0"]
  }
}
`
	path := filepath.Join(g.config.Generator.OutputDir, "terraform-registry-manifest.json")
	return os.WriteFile(path, []byte(content), 0644)
}

// generateGitHubWorkflow creates the GitHub Actions release workflow
func (g *Generator) generateGitHubWorkflow() error {
	tmpl, err := template.ParseFS(templates, "templates/release.yml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse release workflow template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, ".github", "workflows", "release.yml")
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
	}

	return tmpl.Execute(f, data)
}
