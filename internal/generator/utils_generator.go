package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
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

// GenerateSharedTypes generates shared struct definitions from OpenAPI components
func (g *Generator) GenerateSharedTypes() error {
	tmpl, err := template.New("shared_types.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/shared_types.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse shared types template: %w", err)
	}

	var allFields []FieldInfo

	for _, resource := range g.config.Resources {
		ops := resource.OperationIDs()
		var createFields []FieldInfo
		var updateFields []FieldInfo

		if resource.Plugin == "order" {
			// Order resource
			schemaName := strings.ReplaceAll(resource.OfferingType, ".", "") + "CreateOrderAttributes"
			if schema, err := g.parser.GetSchema(schemaName); err == nil {
				if f, err := ExtractFields(schema); err == nil {
					createFields = f
				}
			}
			if op := ops.PartialUpdate; op != "" {
				if schema, err := g.parser.GetOperationRequestSchema(op); err == nil {
					if f, err := ExtractFields(schema); err == nil {
						updateFields = f
					}
				}
			}
		} else {
			// Standard resource
			op := ops.Create
			if resource.LinkOp != "" {
				op = resource.LinkOp
			}
			if resource.CreateOperation != nil && resource.CreateOperation.OperationID != "" {
				op = resource.CreateOperation.OperationID
			}

			if op != "" {
				if schema, err := g.parser.GetOperationRequestSchema(op); err == nil {
					if f, err := ExtractFields(schema); err == nil {
						createFields = f
					}
				}
			}

			if op := ops.PartialUpdate; op != "" {
				if schema, err := g.parser.GetOperationRequestSchema(op); err == nil {
					if f, err := ExtractFields(schema); err == nil {
						updateFields = f
					}
				}
			}
		}

		allFields = append(allFields, createFields...)
		allFields = append(allFields, updateFields...)
	}

	uniqueStructs := collectUniqueStructs(allFields)

	data := map[string]interface{}{
		"Structs": uniqueStructs,
	}

	outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "resources", "shared_types.go")
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return err
	}

	return nil
}
