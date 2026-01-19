package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// createDirectoryStructure creates the output directory structure
func (g *Generator) createDirectoryStructure() error {
	dirs := []string{
		g.config.Generator.OutputDir,
		filepath.Join(g.config.Generator.OutputDir, "internal", "provider"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "resources"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "datasources"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "client"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "testhelpers"),
		filepath.Join(g.config.Generator.OutputDir, "e2e_test", "testdata"),
		filepath.Join(g.config.Generator.OutputDir, "examples"),
		filepath.Join(g.config.Generator.OutputDir, ".github", "workflows"),
		filepath.Join(g.config.Generator.OutputDir, "e2e_test"),
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
	tmpl, err := template.New("provider.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/provider.go.tmpl")
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

	// Generate README.md
	if err := g.generateReadme(); err != nil {
		return err
	}

	// Generate LICENSE
	if err := g.generateLicense(); err != nil {
		return err
	}

	// Generate GitHub Actions workflow
	if err := g.generateGitHubWorkflow(); err != nil {
		return err
	}

	return nil
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
	content := fmt.Sprintf(`module github.com/waldur/terraform-provider-%s

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

// generateReadme creates the README.md file for the generated provider
func (g *Generator) generateReadme() error {
	tmpl, err := template.New("readme.md.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/readme.md.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse readme template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "README.md")
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

// generateLicense copies the LICENSE file from root to output
func (g *Generator) generateLicense() error {
	content, err := os.ReadFile("LICENSE")
	if err != nil {
		return fmt.Errorf("failed to read LICENSE file: %w", err)
	}
	path := filepath.Join(g.config.Generator.OutputDir, "LICENSE")
	return os.WriteFile(path, content, 0644)
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

// generateE2ETests copies E2E tests from templates to output
func (g *Generator) generateE2ETests() error {
	entries, err := templates.ReadDir("templates/e2e")
	if err != nil {
		// It's possible the directory doesn't exist if no tests are there yet
		// We return nil to allow generation to proceed even without E2E tests
		return nil
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "e2e_test")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := templates.ReadFile("templates/e2e/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", entry.Name(), err)
		}

		// Write file
		outputPath := filepath.Join(outputDir, entry.Name())
		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write test file %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// generateVCRHelpers copies VCR helpers from templates to output
func (g *Generator) generateVCRHelpers() error {
	entries, err := templates.ReadDir("templates/testhelpers")
	if err != nil {
		return fmt.Errorf("failed to read templates/testhelpers: %w", err)
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "internal", "testhelpers")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := templates.ReadFile("templates/testhelpers/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", entry.Name(), err)
		}

		// Write file
		outputPath := filepath.Join(outputDir, entry.Name())
		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write helper file %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// generateFixtures copies VCR fixtures from templates to output
func (g *Generator) generateFixtures() error {
	entries, err := templates.ReadDir("templates/fixtures")
	if err != nil {
		// It's possible the directory doesn't exist if no fixtures are there yet
		return nil
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "e2e_test", "testdata")

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := templates.ReadFile("templates/fixtures/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", entry.Name(), err)
		}

		// Write file
		outputPath := filepath.Join(outputDir, entry.Name())
		if err := os.WriteFile(outputPath, content, 0644); err != nil {
			return fmt.Errorf("failed to write fixture file %s: %w", entry.Name(), err)
		}
	}
	return nil
}
