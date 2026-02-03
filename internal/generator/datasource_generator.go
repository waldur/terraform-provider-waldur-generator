package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

func setIsDataSourceRecursive(fields []FieldInfo) {
	for i := range fields {
		fields[i].IsDataSource = true
		fields[i].SchemaSkip = false
		fields[i].Required = false
		fields[i].ReadOnly = true
		if fields[i].ItemSchema != nil {
			fields[i].ItemSchema.IsDataSource = true
			fields[i].ItemSchema.SchemaSkip = false
			fields[i].ItemSchema.Required = false
			fields[i].ItemSchema.ReadOnly = true
			if len(fields[i].ItemSchema.Properties) > 0 {
				setIsDataSourceRecursive(fields[i].ItemSchema.Properties)
			}
		}
		if len(fields[i].Properties) > 0 {
			setIsDataSourceRecursive(fields[i].Properties)
		}
	}
}

// generateDataSourceImplementation generates a data source file
func (g *Generator) generateDataSourceImplementation(rd *ResourceData, dataSource *config.DataSource) error {
	tmpl, err := template.New("datasource.go.tmpl").Funcs(GetFuncMap()).ParseFS(templates, "templates/shared.tmpl", "templates/datasource.go.tmpl")
	if err != nil {
		return fmt.Errorf("failed to parse datasource template: %w", err)
	}

	// For datasources, all fields must be IsDataSource = true
	// We use a clone or just set it on the shared rd fields (if they are already shared, it's fine
	// because we generate the model AFTER this or it doesn't matter for the model anyway).
	// Actually, we should probably cloned if we want to be safe, but ResourceData fields
	// are already specific to this entity.
	setIsDataSourceRecursive(rd.ResponseFields)
	setIsDataSourceRecursive(rd.FilterParams)
	setIsDataSourceRecursive(rd.ModelFields)

	data := map[string]interface{}{
		"Name":           rd.Name,
		"Service":        rd.Service,
		"CleanName":      rd.CleanName,
		"Operations":     rd.Operations,
		"ListPath":       rd.APIPaths["Base"],
		"RetrievePath":   rd.APIPaths["Retrieve"],
		"FilterParams":   rd.FilterParams,
		"ResponseFields": rd.ResponseFields,
		"ModelFields":    rd.ModelFields,
	}

	outputDir := filepath.Join(g.config.Generator.OutputDir, "services", rd.Service, rd.CleanName)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	outputPath := filepath.Join(outputDir, "datasource.go")
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
