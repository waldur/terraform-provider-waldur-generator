package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `generator:
  openapi_schema: "test-schema.yaml"
  output_dir: "./output"
  provider_name: "waldur"

resources:
  - name: "structure_project"
    base_operation_id: "projects"

data_sources:
  - name: "structure_project"
    base_operation_id: "projects"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Test loading
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify config values
	if cfg.Generator.ProviderName != "waldur" {
		t.Errorf("Expected provider_name 'waldur', got '%s'", cfg.Generator.ProviderName)
	}

	if len(cfg.Resources) != 1 {
		t.Errorf("Expected 1 resource, got %d", len(cfg.Resources))
	}

	if cfg.Resources[0].Name != "structure_project" {
		t.Errorf("Expected resource name 'structure_project', got '%s'", cfg.Resources[0].Name)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Generator: GeneratorConfig{
					OpenAPISchema: "schema.yaml",
					ProviderName:  "waldur",
					OutputDir:     "./output",
				},
				Resources: []Resource{
					{Name: "structure_project", BaseOperationID: "projects"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing openapi schema",
			config: &Config{
				Generator: GeneratorConfig{
					ProviderName: "waldur",
				},
			},
			wantErr: true,
		},
		{
			name: "missing provider name",
			config: &Config{
				Generator: GeneratorConfig{
					OpenAPISchema: "schema.yaml",
				},
			},
			wantErr: true,
		},
		{
			name: "duplicate resource name",
			config: &Config{
				Generator: GeneratorConfig{
					OpenAPISchema: "schema.yaml",
					ProviderName:  "waldur",
				},
				Resources: []Resource{
					{Name: "project", BaseOperationID: "projects"},
					{Name: "project", BaseOperationID: "projects"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOperationIDs(t *testing.T) {
	resource := Resource{
		Name:            "structure_project",
		BaseOperationID: "projects",
	}

	ops := resource.OperationIDs()

	expected := map[string]string{
		"List":          "projects_list",
		"Create":        "projects_create",
		"Retrieve":      "projects_retrieve",
		"PartialUpdate": "projects_partial_update",
		"Destroy":       "projects_destroy",
	}

	if ops.List != expected["List"] {
		t.Errorf("Expected List='%s', got '%s'", expected["List"], ops.List)
	}
	if ops.Create != expected["Create"] {
		t.Errorf("Expected Create='%s', got '%s'", expected["Create"], ops.Create)
	}
	if ops.Retrieve != expected["Retrieve"] {
		t.Errorf("Expected Retrieve='%s', got '%s'", expected["Retrieve"], ops.Retrieve)
	}
	if ops.PartialUpdate != expected["PartialUpdate"] {
		t.Errorf("Expected PartialUpdate='%s', got '%s'", expected["PartialUpdate"], ops.PartialUpdate)
	}
	if ops.Destroy != expected["Destroy"] {
		t.Errorf("Expected Destroy='%s', got '%s'", expected["Destroy"], ops.Destroy)
	}
}
