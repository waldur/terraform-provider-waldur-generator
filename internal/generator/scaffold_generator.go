package generator

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
)

// createDirectoryStructure creates the output directory structure
func (g *Generator) createDirectoryStructure() error {
	dirs := []string{
		g.config.Generator.OutputDir,
		filepath.Join(g.config.Generator.OutputDir, "internal", "provider"),
		filepath.Join(g.config.Generator.OutputDir, "services"),
		filepath.Join(g.config.Generator.OutputDir, "internal", "sdk", "common"),
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
	// Collect unique services
	services := make(map[string]bool)
	for _, res := range g.config.Resources {
		service, _ := common.SplitResourceName(res.Name)
		services[service] = true
	}
	for _, ds := range g.config.DataSources {
		service, _ := common.SplitResourceName(ds.Name)
		services[service] = true
	}

	var serviceList []string
	for s := range services {
		serviceList = append(serviceList, s)
	}
	sort.Strings(serviceList)

	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
		"Services":     serviceList,
	}

	return g.RenderTemplate(
		"provider.go.tmpl",
		[]string{"templates/provider.go.tmpl"},
		data,
		filepath.Join(g.config.Generator.OutputDir, "internal", "provider"),
		"provider.go",
	)
}

func (g *Generator) generateServiceRegistrations() error {
	// Group resources by service
	serviceResources := make(map[string][]*common.ResourceData)

	// Process all prepared resources
	for _, name := range g.ResourceOrder {
		rd := g.Resources[name]
		serviceResources[rd.Service] = append(serviceResources[rd.Service], rd)
	}

	for service, resources := range serviceResources {
		outputDir := filepath.Join(g.config.Generator.OutputDir, "services", service)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}

		data := map[string]interface{}{
			"Service":      service,
			"Resources":    resources,
			"ProviderName": g.config.Generator.ProviderName,
		}

		if err := g.RenderTemplate(
			"service_register.go.tmpl",
			[]string{"templates/service_register.go.tmpl"},
			data,
			filepath.Join(g.config.Generator.OutputDir, "services", service),
			"register.go",
		); err != nil {
			return err
		}
	}

	return nil
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

	// Generate examples
	if err := g.generateExamples(); err != nil {
		return err
	}

	return nil
}

// generateMain creates the main.go file for the generated provider
func (g *Generator) generateMain() error {
	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
	}

	return g.RenderTemplate(
		"main.go.tmpl",
		[]string{"templates/main.go.tmpl"},
		data,
		g.config.Generator.OutputDir,
		"main.go",
	)
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
	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
	}

	// Note: We use renderTemplate but template name is .goreleaser.yml which is fine
	return g.RenderTemplate(
		"goreleaser.yml.tmpl",
		[]string{"templates/goreleaser.yml.tmpl"},
		data,
		g.config.Generator.OutputDir,
		".goreleaser.yml",
	)
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
	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
		"Resources":    g.config.Resources,
		"DataSources":  g.config.DataSources,
	}

	return g.RenderTemplate(
		"readme.md.tmpl",
		[]string{"templates/readme.md.tmpl"},
		data,
		g.config.Generator.OutputDir,
		"README.md",
	)
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
	data := map[string]interface{}{
		"ProviderName": g.config.Generator.ProviderName,
	}

	return g.RenderTemplate(
		"release.yml.tmpl",
		[]string{"templates/release.yml.tmpl"},
		data,
		filepath.Join(g.config.Generator.OutputDir, ".github", "workflows"),
		"release.yml",
	)
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
		outputPath := filepath.Join(outputDir, strings.TrimSuffix(entry.Name(), ".tmpl"))
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

// generateExamples generates example files from templates
func (g *Generator) generateExamples() error {
	baseDir := "templates/examples"
	return fs.WalkDir(templates, baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Calculate output path
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Map templates/... to examples/...
		outputPath := filepath.Join(g.config.Generator.OutputDir, "examples", relPath)
		outputPath = strings.TrimSuffix(outputPath, ".tmpl")

		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return err
		}

		if strings.HasSuffix(path, ".tmpl") {
			// Execute template
			tmpl, err := template.New(filepath.Base(path)).Funcs(GetFuncMap()).ParseFS(templates, path)
			if err != nil {
				return fmt.Errorf("failed to parse template %s: %w", path, err)
			}

			f, err := os.Create(outputPath)
			if err != nil {
				return err
			}
			defer f.Close()

			data := map[string]interface{}{
				"ProviderName": g.config.Generator.ProviderName,
			}
			if err := tmpl.Execute(f, data); err != nil {
				return fmt.Errorf("failed to execute template %s: %w", path, err)
			}
		} else {
			// Just copy
			content, err := templates.ReadFile(path)
			if err != nil {
				return err
			}
			if err := os.WriteFile(outputPath, content, 0644); err != nil {
				return err
			}
		}
		return nil
	})
}
