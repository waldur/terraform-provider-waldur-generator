package generator

import (
	"fmt"
	"os"
	"path/filepath"
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
		data := map[string]interface{}{
			"ResourceName":    resource.Name,
			"ActionName":      actionName,
			"OperationID":     operationID,
			"BaseOperationID": resource.BaseOperationID,
			"Description":     fmt.Sprintf("Perform %s action on %s", actionName, resource.Name),
			"IdentifierParam": "uuid", // Default identifier
			"IdentifierDesc":  fmt.Sprintf("The UUID of the %s", resource.Name),
			"ProviderName":    g.config.Generator.ProviderName,
			"Path":            path,
			"Method":          method,
		}

		// Generate file
		filename := fmt.Sprintf("%s_%s.go", resource.Name, actionName)
		outputPath := filepath.Join(g.config.Generator.OutputDir, "internal", "actions", filename)

		// Ensure directory exists
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
	}

	return nil
}
