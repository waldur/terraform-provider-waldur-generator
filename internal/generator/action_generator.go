package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

func (g *Generator) generateActions(resource *config.Resource) error {
	tmpl, err := template.New("action.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/action.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse action template: %w", err)
	}

	for _, actionName := range resource.Actions {
		// Construct operation ID: typically "base_operation_id" + "_" + actionName
		// e.g., marketplace_resources_pull
		operationID := fmt.Sprintf("%s_%s", resource.BaseOperationID, actionName)

		// Validate operation exists
		method, path, _, err := g.parser.GetOperation(operationID)
		if err != nil {
			return fmt.Errorf("action %s operation %s not found: %w", actionName, operationID, err)
		}

		// Prepare data for template
		service, cleanName := splitResourceName(resource.Name)
		resourceName := strings.ReplaceAll(resource.Name, "_", " ")
		data := map[string]interface{}{
			"ResourceName":    resource.Name,
			"Service":         service,
			"CleanName":       cleanName,
			"ActionName":      actionName,
			"OperationID":     operationID,
			"BaseOperationID": resource.BaseOperationID,
			"Description":     fmt.Sprintf("Perform %s action on %s", actionName, resourceName),
			"IdentifierParam": "uuid", // Default identifier
			"IdentifierDesc":  fmt.Sprintf("The UUID of the %s", resourceName),
			"ProviderName":    g.config.Generator.ProviderName,
			"Path":            path,
			"Method":          method,
		}

		// Generate file
		outputDir := filepath.Join(g.config.Generator.OutputDir, "services", service, cleanName)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return err
		}

		outputPath := filepath.Join(outputDir, fmt.Sprintf("%s.go", actionName))
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
