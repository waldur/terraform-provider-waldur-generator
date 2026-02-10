package generator

import (
	"embed"
	"fmt"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
	"github.com/waldur/terraform-provider-waldur-generator/internal/generator/common"
	"github.com/waldur/terraform-provider-waldur-generator/internal/openapi"
)

//go:embed templates/* plugins/*
var templates embed.FS

// Generator orchestrates the provider code generation
type Generator struct {
	config *config.Config
	parser *openapi.Parser
}

// ResourceData holds all data required to generate resource/sdk code
type ResourceData = common.ResourceData

// FieldInfo represents information about a field extracted from OpenAPI schema
type FieldInfo = common.FieldInfo

// UpdateAction represents an enriched update action with resolved API path
type UpdateAction = common.UpdateAction

// FilterParam describes a query parameter for filtering
type FilterParam = common.FilterParam

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

	// 1. Prepare data
	mergedResources := make(map[string]*ResourceData)
	var resourceOrder []string

	for i := range g.config.Resources {
		res := &g.config.Resources[i]
		rd, err := g.prepareResourceData(res)
		if err != nil {
			return err
		}
		mergedResources[res.Name] = rd
		resourceOrder = append(resourceOrder, res.Name)
	}

	for i := range g.config.DataSources {
		ds := &g.config.DataSources[i]
		dd, err := g.prepareDatasourceData(ds)
		if err != nil {
			return err
		}

		if existing, ok := mergedResources[ds.Name]; ok {
			// Merge datasource fields into existing resource data
			existing.ResponseFields = common.MergeFields(existing.ResponseFields, dd.ResponseFields)
			existing.ModelFields = common.MergeFields(existing.ModelFields, dd.ModelFields)
			existing.HasDataSource = true
			if dd.APIPaths != nil {
				if existing.APIPaths == nil {
					existing.APIPaths = make(map[string]string)
				}
				for k, v := range dd.APIPaths {
					if _, exists := existing.APIPaths[k]; !exists {
						existing.APIPaths[k] = v
					}
				}
			}
		} else {
			mergedResources[ds.Name] = dd
			resourceOrder = append(resourceOrder, ds.Name)
		}
	}

	// 2. Generate provider files
	if err := g.generateProvider(); err != nil {
		return fmt.Errorf("failed to generate provider: %w", err)
	}

	// 3. Generate service registration files
	if err := g.generateServiceRegistrations(); err != nil {
		return fmt.Errorf("failed to generate service registrations: %w", err)
	}

	// 4. Generate implementation for all entities
	for _, name := range resourceOrder {
		rd := mergedResources[name]

		// Hack: Ensure target_tenant is present for openstack_port and NOT skipped
		if name == "openstack_port" {
			foundIndex := -1
			for i, f := range rd.ResponseFields {
				if f.Name == "target_tenant" {
					foundIndex = i
					break
				}
			}
			if foundIndex != -1 {
				rd.ResponseFields[foundIndex].SchemaSkip = false
			} else {
				rd.ResponseFields = append(rd.ResponseFields, FieldInfo{
					Name:        "target_tenant",
					GoType:      "types.String",
					Type:        "string",
					Description: "Target Tenant UUID",
					SchemaSkip:  false,
				})
			}
		}

		// Generate model once for the entity
		if err := g.generateModel(rd); err != nil {
			return fmt.Errorf("failed to generate model for %s: %w", name, err)
		}

		// Generate SDK components
		if err := g.generateResourceSDK(rd); err != nil {
			return fmt.Errorf("failed to generate SDK for %s: %w", name, err)
		}

		// If it has a resource configuration, generate it
		if !rd.IsDatasourceOnly {
			var configRes *config.Resource
			for i := range g.config.Resources {
				if g.config.Resources[i].Name == name {
					configRes = &g.config.Resources[i]
					break
				}
			}
			if configRes != nil && configRes.Plugin != "actions" {
				if err := g.generateResourceImplementation(rd); err != nil {
					return fmt.Errorf("failed to generate resource implementation %s: %w", name, err)
				}
				if err := g.generateListResourceImplementation(rd); err != nil {
					fmt.Printf("Warning: failed to generate list resource %s: %s\n", name, err)
				}

				// Actions
				if len(configRes.Actions) > 0 {
					if err := g.generateActionsImplementation(rd); err != nil {
						return fmt.Errorf("failed to generate actions for resource %s: %w", name, err)
					}
				}
			}
		}

		// If it has a datasource configuration, generate it
		for i := range g.config.DataSources {
			if g.config.DataSources[i].Name == name {
				if err := g.generateDataSourceImplementation(rd, &g.config.DataSources[i]); err != nil {
					return fmt.Errorf("failed to generate data source %s: %w", name, err)
				}
			}
		}
	}

	// 5. Generate supporting files
	if err := g.generateSupportingFiles(); err != nil {
		return fmt.Errorf("failed to generate supporting files: %w", err)
	}

	// 6. Generate shared utils
	if err := g.generateSharedUtils(); err != nil {
		return fmt.Errorf("failed to generate shared utils: %w", err)
	}

	// 7. Generate shared SDK types
	if err := g.generateSharedSDKTypes(); err != nil {
		return fmt.Errorf("failed to generate shared types: %w", err)
	}

	// 8. Generate E2E tests
	if err := g.generateE2ETests(); err != nil {
		return fmt.Errorf("failed to generate E2E tests: %w", err)
	}

	// 9. Generate VCR helpers
	if err := g.generateVCRHelpers(); err != nil {
		return fmt.Errorf("failed to generate VCR helpers: %w", err)
	}

	// 10. Generate VCR fixtures
	if err := g.generateFixtures(); err != nil {
		return fmt.Errorf("failed to generate VCR fixtures: %w", err)
	}

	// 11. Clean up generated Go files (format and remove unused imports)
	if err := g.cleanupImports(); err != nil {
		return fmt.Errorf("failed to cleanup imports: %w", err)
	}

	return nil
}

func (g *Generator) hasDataSource(resourceName string) bool {
	for _, ds := range g.config.DataSources {
		if ds.Name == resourceName {
			return true
		}
	}
	return false
}
