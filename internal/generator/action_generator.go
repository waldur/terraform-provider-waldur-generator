package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func (g *Generator) generateActionsImplementation(rd *ResourceData) error {
	tmpl, err := template.New("action.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/action.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse action template: %w", err)
	}

	for _, action := range rd.StandaloneActions {
		// Validate operation exists
		method, path, _, err := g.parser.GetOperation(action.Operation)
		if err != nil {
			return fmt.Errorf("action %s operation %s not found: %w", action.Name, action.Operation, err)
		}

		// Prepare data for template
		resourceName := strings.ReplaceAll(rd.Name, "_", " ")
		data := map[string]interface{}{
			"ResourceName":    rd.Name,
			"Service":         rd.Service,
			"CleanName":       rd.CleanName,
			"ActionName":      action.Name,
			"OperationID":     action.Operation,
			"BaseOperationID": rd.BaseOperationID,
			"Description":     fmt.Sprintf("Perform %s action on %s", action.Name, resourceName),
			"IdentifierParam": "uuid", // Default identifier
			"IdentifierDesc":  fmt.Sprintf("The UUID of the %s", resourceName),
			"ProviderName":    g.config.Generator.ProviderName,
			"Path":            path,
			"Method":          method,
		}

		// Generate file
		outputDir := filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}

		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.go", action.Name))
		f, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer f.Close()

		if err := tmpl.Execute(f, data); err != nil {
			return err
		}
	}

	return nil
}
