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

// ToSnakeCase converts a string to snake_case
func ToSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// displayName strips module prefix (anything before first underscore) and converts to title case for user-facing messages
func displayName(s string) string {
	// Strip everything before first underscore (e.g., "structure_project" -> "project")
	name := s
	if idx := strings.Index(s, "_"); idx != -1 {
		name = s[idx+1:]
	}

	// Convert to title case
	return toTitle(name)
}

// FilterParam describes a query parameter for filtering
type FilterParam struct {
	Name        string
	TFSDKName   string
	Type        string // String, Int64, Bool, Float64
	Description string
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

// validateOperations checks that all referenced operations exist in the OpenAPI schema
func (g *Generator) validateOperations() error {
	for _, resource := range g.config.Resources {
		ops := resource.OperationIDs()

		// For order resources, create and destroy operations don't exist
		// (they use marketplace-orders API instead)
		operationsToCheck := []string{ops.List, ops.Retrieve, ops.PartialUpdate}
		if resource.Plugin != "order" {
			operationsToCheck = append(operationsToCheck, ops.Create, ops.Destroy)
		}

		for _, opID := range operationsToCheck {
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
	tmpl, err := template.New("provider.go.tmpl").Funcs(template.FuncMap{
		"title":       toTitle,
		"displayName": displayName,
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

// getFuncMap returns the common template functions
func (g *Generator) getFuncMap() template.FuncMap {
	return template.FuncMap{
		"title":       toTitle,
		"displayName": displayName,
		"sanitize": func(s string) string {
			// Replace problematic characters in descriptions
			s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslashes first
			s = strings.ReplaceAll(s, "\"", "\\\"") // Escape quotes
			s = strings.ReplaceAll(s, "\n", " ")    // Replace newlines with spaces
			s = strings.ReplaceAll(s, "\r", "")     // Remove carriage returns
			s = strings.ReplaceAll(s, "\t", " ")    // Replace tabs with spaces
			// Normalize multiple spaces
			for strings.Contains(s, "  ") {
				s = strings.ReplaceAll(s, "  ", " ")
			}
			return strings.TrimSpace(s)
		},
		"toAttrType": ToAttrType,
		"len": func(v interface{}) int {
			// Handle different types if needed, for now assume []FieldInfo
			if fields, ok := v.([]FieldInfo); ok {
				return len(fields)
			}
			return 0
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments")
			}
			result := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				result[key] = values[i+1]
			}
			return result, nil
		},
	}
}

// ToAttrType converts FieldInfo to proper attr.Type expression used in Terraform schema
func ToAttrType(f FieldInfo) string {
	switch f.GoType {
	case "types.String":
		return "types.StringType"
	case "types.Int64":
		return "types.Int64Type"
	case "types.Bool":
		return "types.BoolType"
	case "types.Float64":
		return "types.Float64Type"
	case "types.List":
		// For lists, we need to specify the element type
		if f.ItemType == "string" {
			return "types.ListType{ElemType: types.StringType}"
		} else if f.ItemType == "integer" {
			return "types.ListType{ElemType: types.Int64Type}"
		} else if f.ItemType == "object" && f.ItemSchema != nil {
			// Recursively build the object type for list items
			var attrs []string
			for _, prop := range f.ItemSchema.Properties {
				attrs = append(attrs, fmt.Sprintf("%q: %s", prop.TFSDKName, ToAttrType(prop)))
			}
			return fmt.Sprintf("types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{%s}}}", strings.Join(attrs, ", "))
		}
		return "types.ListType{ElemType: types.StringType}" // Default
	case "types.Object":
		// For objects, build the attribute types map
		var attrs []string
		for _, prop := range f.Properties {
			attrs = append(attrs, fmt.Sprintf("%q: %s", prop.TFSDKName, ToAttrType(prop)))
		}
		return fmt.Sprintf("types.ObjectType{AttrTypes: map[string]attr.Type{%s}}", strings.Join(attrs, ", "))
	default:
		return "types.StringType" // Fallback
	}
}

// generateResource generates a resource file
func (g *Generator) generateResource(resource *config.Resource) error {
	tmpl, err := template.New("resource.go.tmpl").Funcs(g.getFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/resource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse resource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", fmt.Sprintf("%s.go", resource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	ops := resource.OperationIDs()

	// Extract API paths from OpenAPI operations
	apiPaths := make(map[string]string)

	// Get path from list operation (used as base path)
	if _, listPath, _, err := g.parser.GetOperation(ops.List); err == nil {
		// Remove trailing slash and {uuid} if present for base path
		basePath := listPath
		apiPaths["Base"] = basePath
	}

	// Get path from create operation
	if _, createPath, _, err := g.parser.GetOperation(ops.Create); err == nil {
		apiPaths["Create"] = createPath
	}

	// Get path from retrieve operation (includes UUID parameter)
	if _, retrievePath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		apiPaths["Retrieve"] = retrievePath
	}

	// Get path from update operation
	if _, updatePath, _, err := g.parser.GetOperation(ops.PartialUpdate); err == nil {
		apiPaths["Update"] = updatePath
	}

	// Get path from delete operation
	if _, deletePath, _, err := g.parser.GetOperation(ops.Destroy); err == nil {
		apiPaths["Delete"] = deletePath
	}

	// Extract fields
	var createFields []FieldInfo
	var updateFields []FieldInfo
	var responseFields []FieldInfo
	var modelFields []FieldInfo

	isOrder := resource.Plugin == "order"

	if isOrder {
		// Order resource logic
		// 1. Get Offering Schema (Input)
		// Remove dots from offering type for schema name (e.g. OpenStack.Instance -> OpenStackInstanceCreateOrderAttributes)
		schemaName := strings.ReplaceAll(resource.OfferingType, ".", "") + "CreateOrderAttributes"

		offeringSchema, err := g.parser.GetSchema(schemaName)
		if err != nil {
			return fmt.Errorf("failed to find offering schema %s: %w", schemaName, err)
		}

		if fields, err := ExtractFields(offeringSchema); err == nil {
			createFields = fields
			// Mark all plugin fields as optional to allow system-populated values
			// and delegate validation to the API
			for i := range createFields {
				createFields[i].Required = false
			}
		}

		// 2. Get Resource Schema (Output) from Retrieve operation
		if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		}

		// 3. Merge fields
		modelFields = MergeOrderFields(createFields, responseFields)

		// 4. Add Termination Attributes
		for _, term := range resource.TerminationAttributes {
			goType := "types.String"
			switch term.Type {
			case "boolean":
				goType = "types.Bool"
			case "integer":
				goType = "types.Int64"
			case "number":
				goType = "types.Float64"
			}

			modelFields = append(modelFields, FieldInfo{
				Name:        term.Name,
				Type:        term.Type,
				Description: "Termination attribute",
				GoType:      goType,
				TFSDKName:   ToSnakeCase(term.Name),
				// Required: false, ReadOnly: false -> Optional: true
			})
		}

		// Extract Update fields from Resource PartialUpdate operation
		if updateSchema, err := g.parser.GetOperationRequestSchema(ops.PartialUpdate); err == nil {
			if fields, err := ExtractFields(updateSchema); err == nil {
				updateFields = fields
			}
		}

	} else {
		// Standard resource logic
		// Extract Create fields
		if createSchema, err := g.parser.GetOperationRequestSchema(ops.Create); err == nil {
			if fields, err := ExtractFields(createSchema); err == nil {
				createFields = fields
			}
		}

		// Extract Update fields
		if updateSchema, err := g.parser.GetOperationRequestSchema(ops.PartialUpdate); err == nil {
			if fields, err := ExtractFields(updateSchema); err == nil {
				updateFields = fields
			}
		}

		// Extract Response fields (prefer Retrieve operation as it's usually most complete)
		if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		} else if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Create); err == nil {
			// Fallback to Create response
			if fields, err := ExtractFields(responseSchema); err == nil {
				responseFields = fields
			}
		}

		// Merge fields for the model (Create + Response)
		modelFields = MergeFields(createFields, responseFields)
	}

	// Update responseFields to use merged field definitions
	// This ensures shared.tmpl uses the complete schema for nested objects
	modelMap := make(map[string]FieldInfo)
	for _, f := range modelFields {
		modelMap[f.Name] = f
	}
	var newResponseFields []FieldInfo
	for _, f := range responseFields {
		if mergedF, ok := modelMap[f.Name]; ok {
			newResponseFields = append(newResponseFields, mergedF)
		} else {
			newResponseFields = append(newResponseFields, f)
		}
	}
	responseFields = newResponseFields

	data := map[string]interface{}{
		"Name":                  resource.Name,
		"Operations":            ops,
		"APIPaths":              apiPaths,
		"CreateFields":          createFields,
		"UpdateFields":          updateFields,
		"ResponseFields":        responseFields,
		"ModelFields":           modelFields,
		"IsOrder":               isOrder,
		"OfferingType":          resource.OfferingType,
		"UpdateActions":         resource.UpdateActions,
		"TerminationAttributes": resource.TerminationAttributes,
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}

// generateDataSource generates a data source file
func (g *Generator) generateDataSource(dataSource *config.DataSource) error {
	tmpl, err := template.New("datasource.go.tmpl").Funcs(g.getFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/datasource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse datasource template: %w", err)
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "datasources", fmt.Sprintf("%s.go", dataSource.Name))
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	ops := dataSource.OperationIDs()

	// Extract API paths from OpenAPI operations
	listPath := ""
	retrievePath := ""

	// Use list path as primary since it's needed for filtering
	if _, path, _, err := g.parser.GetOperation(ops.List); err == nil {
		listPath = path
	} else if _, retPath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		// Fall back to retrieve path if list doesn't exist
		listPath = retPath
	}

	// Also get retrieve path separately for UUID lookups
	if _, retPath, _, err := g.parser.GetOperation(ops.Retrieve); err == nil {
		retrievePath = retPath
	}

	// Extract query parameters from list operation
	var filterParams []FilterParam
	if operation, _, _, err := g.parser.GetOperation(ops.List); err == nil {
		for _, param := range operation.Parameters {
			if param.Value != nil && param.Value.In == "query" {
				paramName := param.Value.Name

				// Skip pagination and ordering parameters
				if paramName == "page" || paramName == "page_size" || paramName == "o" {
					continue
				}

				description := param.Value.Description

				// Determine Terraform type
				tfType := "String" // Default
				if param.Value.Schema != nil && param.Value.Schema.Value != nil {
					// Get the Go type string (e.g. "types.Int64") from the OpenAPI type
					goType := GetGoType(getSchemaType(param.Value.Schema.Value))
					// Convert it to the simple string identifier used in FilterParam
					tfType = GetFilterParamType(goType)
				}

				filterParams = append(filterParams, FilterParam{
					Name:        paramName,
					TFSDKName:   ToSnakeCase(paramName), // Ensure valid TF attribute name
					Type:        tfType,
					Description: description,
				})
			}
		}
	}

	// Add manually configured filter params
	existingFilters := make(map[string]bool)
	for _, fp := range filterParams {
		existingFilters[fp.Name] = true
	}

	for _, paramName := range dataSource.FilterParams {
		if !existingFilters[paramName] {
			filterParams = append(filterParams, FilterParam{
				Name:        paramName,
				TFSDKName:   ToSnakeCase(paramName),
				Type:        "String",
				Description: "Filter by " + paramName,
			})
			existingFilters[paramName] = true
		}
	}

	// Extract Response fields from Retrieve operation
	var responseFields []FieldInfo
	if responseSchema, err := g.parser.GetOperationResponseSchema(ops.Retrieve); err == nil {
		if fields, err := ExtractFields(responseSchema); err == nil {
			responseFields = fields
		}
	} else if responseSchema, err := g.parser.GetOperationResponseSchema(ops.List); err == nil {
		// For list, the schema is usually an array of items. We need the item schema.
		if responseSchema.Value.Type != nil && (*responseSchema.Value.Type)[0] == "array" && responseSchema.Value.Items != nil {
			if fields, err := ExtractFields(responseSchema.Value.Items); err == nil {
				responseFields = fields
			}
		}
	}

	// Deduplicate: remove ResponseFields that exist in FilterParams
	filterNames := make(map[string]bool)
	for _, fp := range filterParams {
		filterNames[fp.Name] = true
	}
	var dedupedResponseFields []FieldInfo
	for _, rf := range responseFields {
		if !filterNames[rf.Name] {
			dedupedResponseFields = append(dedupedResponseFields, rf)
		}
	}

	data := map[string]interface{}{
		"Name":           dataSource.Name,
		"Operations":     ops,
		"ListPath":       listPath,
		"RetrievePath":   retrievePath,
		"FilterParams":   filterParams,
		"ResponseFields": dedupedResponseFields, // Use deduped version
		"ModelFields":    dedupedResponseFields, // Map to ModelFields for shared template compatibility
	}

	if err := tmpl.Execute(f, data); err != nil {
		return err
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
