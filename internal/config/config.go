package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the generator configuration
type Config struct {
	Generator   GeneratorConfig `yaml:"generator"`
	Resources   []Resource      `yaml:"resources"`
	DataSources []DataSource    `yaml:"data_sources"`
}

// GeneratorConfig contains global generator settings
type GeneratorConfig struct {
	OpenAPISchema string `yaml:"openapi_schema"`
	OutputDir     string `yaml:"output_dir"`
	ProviderName  string `yaml:"provider_name"`
}

// Resource defines a Terraform resource to generate
type Resource struct {
	Name                  string                        `yaml:"name"`
	BaseOperationID       string                        `yaml:"base_operation_id"`
	Plugin                string                        `yaml:"plugin"`
	OfferingType          string                        `yaml:"offering_type"`
	UpdateActions         map[string]UpdateActionConfig `yaml:"update_actions"`
	TerminationAttributes []ParameterConfig             `yaml:"termination_attributes"`
}

// UpdateActionConfig defines a custom update action
type UpdateActionConfig struct {
	Param     string `yaml:"param"`
	Operation string `yaml:"operation"`
}

// ParameterConfig defines a parameter configuration
type ParameterConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

// DataSource defines a Terraform data source to generate
type DataSource struct {
	Name            string `yaml:"name"`
	BaseOperationID string `yaml:"base_operation_id"`
}

// OperationIDs returns the inferred operation IDs for a resource
func (r *Resource) OperationIDs() OperationSet {
	return OperationSet{
		List:          r.BaseOperationID + "_list",
		Create:        r.BaseOperationID + "_create",
		Retrieve:      r.BaseOperationID + "_retrieve",
		PartialUpdate: r.BaseOperationID + "_partial_update",
		Destroy:       r.BaseOperationID + "_destroy",
	}
}

// OperationIDs returns the inferred operation IDs for a data source
func (d *DataSource) OperationIDs() OperationSet {
	return OperationSet{
		List:     d.BaseOperationID + "_list",
		Retrieve: d.BaseOperationID + "_retrieve",
	}
}

// OperationSet represents the set of operations for a resource
type OperationSet struct {
	List          string
	Create        string
	Retrieve      string
	PartialUpdate string
	Destroy       string
}

// LoadConfig reads and parses the configuration file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if config.Generator.OutputDir == "" {
		config.Generator.OutputDir = "./output/terraform-waldur-provider"
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Generator.OpenAPISchema == "" {
		return fmt.Errorf("openapi_schema is required")
	}
	if c.Generator.ProviderName == "" {
		return fmt.Errorf("provider_name is required")
	}

	// Check for duplicate resource names
	resourceNames := make(map[string]bool)
	for _, r := range c.Resources {
		if r.Name == "" {
			return fmt.Errorf("resource name cannot be empty")
		}
		if r.BaseOperationID == "" {
			return fmt.Errorf("resource %s: base_operation_id cannot be empty", r.Name)
		}
		if resourceNames[r.Name] {
			return fmt.Errorf("duplicate resource name: %s", r.Name)
		}
		resourceNames[r.Name] = true
	}

	// Check for duplicate data source names (separate namespace from resources)
	dataSourceNames := make(map[string]bool)
	for _, d := range c.DataSources {
		if d.Name == "" {
			return fmt.Errorf("data source name cannot be empty")
		}
		if d.BaseOperationID == "" {
			return fmt.Errorf("data source %s: base_operation_id cannot be empty", d.Name)
		}
		if dataSourceNames[d.Name] {
			return fmt.Errorf("duplicate data source name: %s", d.Name)
		}
		dataSourceNames[d.Name] = true
	}

	return nil
}
