package generator

import (
	"embed"
	"fmt"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

//go:embed templates/*
var templates embed.FS

// Generator orchestrates the provider code generation
type Generator struct {
	config *config.Config
	parser *openapi.Parser
}

// ResourceData holds all data required to generate resource/sdk code
type ResourceData struct {
	Name                  string
	Plugin                string
	CheckingLink          bool
	Operations            config.OperationSet
	APIPaths              map[string]string
	CreateFields          []FieldInfo
	UpdateFields          []FieldInfo
	ResponseFields        []FieldInfo
	ModelFields           []FieldInfo
	IsOrder               bool
	IsLink                bool
	IsDatasourceOnly      bool // True if this is a datasource-only definition (no resource)
	Source                *config.LinkResourceConfig
	Target                *config.LinkResourceConfig
	LinkCheckKey          string
	OfferingType          string
	UpdateActions         []UpdateAction
	StandaloneActions     []UpdateAction
	TerminationAttributes []config.ParameterConfig
	CreateOperation       *config.CreateOperationConfig
	CompositeKeys         []string
	NestedStructs         []FieldInfo // Only used for legacy resource generation if needed
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
		// Generate resource implementation
		if resource.Plugin != "actions" {
			if err := g.generateResource(&resource); err != nil {
				return fmt.Errorf("failed to generate resource %s: %w", resource.Name, err)
			}
			if err := g.generateListResource(&resource); err != nil {
				// Log warning but don't fail, as some resources might not have list operations
				fmt.Printf("Warning: failed to generate list resource %s: %s\n", resource.Name, err)
			}
		}

		// Generate actions if defined
		if len(resource.Actions) > 0 {
			if err := g.generateActions(&resource); err != nil {
				return fmt.Errorf("failed to generate actions for resource %s: %w", resource.Name, err)
			}
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

	// Generate shared utils
	if err := g.generateSharedUtils(); err != nil {
		return fmt.Errorf("failed to generate shared utils: %w", err)
	}

	// Generate shared types (OpenAPI components)
	if err := g.GenerateSharedTypes(); err != nil {
		return fmt.Errorf("failed to generate shared types: %w", err)
	}

	// Generate SDK
	if err := g.GenerateSDK(); err != nil {
		return fmt.Errorf("failed to generate sdk: %w", err)
	}

	// Generate E2E tests
	if err := g.generateE2ETests(); err != nil {
		return fmt.Errorf("failed to generate E2E tests: %w", err)
	}

	// Generate VCR helpers
	if err := g.generateVCRHelpers(); err != nil {
		return fmt.Errorf("failed to generate VCR helpers: %w", err)
	}

	// Generate VCR fixtures
	if err := g.generateFixtures(); err != nil {
		return fmt.Errorf("failed to generate VCR fixtures: %w", err)
	}

	// Clean up generated Go files (format and remove unused imports)
	if err := g.cleanupImports(); err != nil {
		return fmt.Errorf("failed to cleanup imports: %w", err)
	}

	return nil
}
