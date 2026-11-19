package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the generator configuration
type Config struct {
	Generator          GeneratorConfig     `yaml:"generator"`
	Resources          []Resource          `yaml:"resources"`
	DataSources        []DataSource        `yaml:"data_sources"`
	EphemeralResources []EphemeralResource `yaml:"ephemeral_resources"`
}

// GeneratorConfig contains global generator settings
type GeneratorConfig struct {
	OpenAPISchema  string   `yaml:"openapi_schema"`
	OutputDir      string   `yaml:"output_dir"`
	ProviderName   string   `yaml:"provider_name"`
	ExcludedFields []string `yaml:"excluded_fields"`
	SetFields      []string `yaml:"set_fields"`
}

// Resource defines a Terraform resource to generate
type Resource struct {
	Name                  string                        `yaml:"name"`
	BaseOperationID       string                        `yaml:"base_operation_id"`
	Plugin                string                        `yaml:"plugin"`
	OfferingType          string                        `yaml:"offering_type"`
	UpdateActions         map[string]UpdateActionConfig `yaml:"update_actions"`
	TerminationAttributes []ParameterConfig             `yaml:"termination_attributes"`
	SkipOperations        []string                      `yaml:"skip_operations"`  // Operations to skip validation for
	CreateOperation       *CreateOperationConfig        `yaml:"create_operation"` // Custom create operation (for nested resources)
	CompositeKeys         []string                      `yaml:"composite_keys"`   // Fields that together form a unique identifier
	// Link Plugin Fields
	Source         *LinkResourceConfig    `yaml:"source"`
	Target         *LinkResourceConfig    `yaml:"target"`
	LinkOp         string                 `yaml:"link_op"`
	UnlinkOp       string                 `yaml:"unlink_op"`
	LinkCheckKey   string                 `yaml:"link_check_key"` // Key in source resource to check for target presence
	LinkParams     []ParameterConfig      `yaml:"link_params"`    // Additional parameters for link operation
	Actions        []string               `yaml:"actions"`        // List of actions to generate (for "actions" plugin)
	SetFields      map[string]FieldConfig `yaml:"set_fields"`
	ExcludedFields []string               `yaml:"excluded_fields"`
	IncludedFields []string               `yaml:"included_fields"` // Global excluded_fields to keep for this resource
	// Scoped Permission Plugin Fields
	ScopeType  string `yaml:"scope_type"`  // API collection prefix, e.g. "projects", "customers"
	ScopeField string `yaml:"scope_field"` // Terraform attribute name for the scope UUID, e.g. "project", "customer"
}

// FieldConfig defines overrides for a field
type FieldConfig struct {
	Computed      bool `yaml:"computed"`
	Optional      bool `yaml:"optional"`
	Required      bool `yaml:"required"`
	ForceNew      bool `yaml:"force_new"`
	Set           bool `yaml:"set"` // True if field should be a Set instead of List
	UnknownIfNull bool `yaml:"unknown_if_null"`
}

// LinkResourceConfig defines configuration for a linked resource
type LinkResourceConfig struct {
	Param      string `yaml:"param"`       // Parameter name in link operation
	RetrieveOp string `yaml:"retrieve_op"` // Operation to retrieve the resource state
}

// CreateOperationConfig defines a custom create operation for nested resources
type CreateOperationConfig struct {
	OperationID string            `yaml:"operation_id"` // The OpenAPI operation ID (e.g., "openstack_tenants_create_floating_ip")
	PathParams  map[string]string `yaml:"path_params"`  // Path parameters mapping (e.g., uuid: tenant)
}

// UpdateActionConfig defines a custom update action
type UpdateActionConfig struct {
	Operation    string `yaml:"operation"`     // The OpenAPI operation ID (e.g., "marketplace_resources_update_limits")
	Param        string `yaml:"param"`         // The Terraform attribute whose change triggers the action
	RequestParam string `yaml:"request_param"` // The key in the action's request body (defaults to Param)
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
	// PluralName, when set, also generates a data source of that name which
	// returns every object the filters match as a list under `items`, where the
	// singular data source insists on exactly one.
	PluralName string `yaml:"plural_name"`
}

// EphemeralResource defines a Terraform ephemeral resource: a value fetched by
// one API operation on every run and never written to the plan or the state.
// It suits secrets an operation reveals, such as an API key.
type EphemeralResource struct {
	Name        string `yaml:"name"`         // Terraform type name, without the provider prefix
	OperationID string `yaml:"operation_id"` // GET or POST operation without a request body
	// PathParams maps each path parameter of the operation to the required
	// Terraform argument that supplies it (e.g. uuid: id).
	PathParams map[string]string `yaml:"path_params"`
	// SensitiveFields are response attributes marked sensitive, so Terraform
	// redacts them wherever they are shown.
	SensitiveFields []string `yaml:"sensitive_fields"`
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
		config.Generator.OutputDir = "./output/terraform-provider-waldur"
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
		if r.BaseOperationID == "" && r.Plugin != "identity_bridge" && r.Plugin != "project_permission" && r.Plugin != "scoped_permission" {
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
	for _, d := range c.DataSources {
		if d.PluralName == "" {
			continue
		}
		if d.PluralName == d.Name || dataSourceNames[d.PluralName] {
			return fmt.Errorf("data source %s: plural_name %s is already a data source name", d.Name, d.PluralName)
		}
		dataSourceNames[d.PluralName] = true
	}

	// Ephemeral resources have their own namespace in Terraform: one may share
	// its name with a data source or a resource.
	ephemeralNames := make(map[string]bool)
	for _, e := range c.EphemeralResources {
		if e.Name == "" {
			return fmt.Errorf("ephemeral resource name cannot be empty")
		}
		if e.OperationID == "" {
			return fmt.Errorf("ephemeral resource %s: operation_id cannot be empty", e.Name)
		}
		if ephemeralNames[e.Name] {
			return fmt.Errorf("duplicate ephemeral resource name: %s", e.Name)
		}
		ephemeralNames[e.Name] = true
	}

	return nil
}
