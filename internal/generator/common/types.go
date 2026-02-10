package common

import (
	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// FieldInfo represents information about a field extracted from OpenAPI schema
type FieldInfo struct {
	Name               string // JSON field name, e.g., "name"
	Type               string // OpenAPI type: "string", "integer", "boolean", "number", "array", "object"
	Required           bool   // Whether field is in schema.Required array
	ReadOnly           bool   // Whether field is marked readOnly in schema
	Description        string // Field description from schema
	Format             string // OpenAPI format: "date-time", "uuid", etc.
	GoType             string // Terraform Framework type: "types.String", "types.List", "types.Object", etc.
	ForceNew           bool   // Whether field requires replacement on change (immutable)
	ServerComputed     bool   // Whether value can be set by server (readOnly or response-only)
	UseStateForUnknown bool   // Whether to use UseStateForUnknown plan modifier
	IsPathParam        bool   // Whether field is a path parameter (should not be in JSON body)

	// Complex type support
	Enum       []string    // For enums: allowed values (only for string type)
	ItemType   string      // For arrays: type of items ("string", "integer", "object", etc.)
	ItemSchema *FieldInfo  // For arrays of objects: nested schema
	Properties []FieldInfo // For nested objects: object properties

	// Validation support
	Minimum *float64 // Minimum value for numeric fields
	Maximum *float64 // Maximum value for numeric fields
	Pattern string   // Regex pattern for string fields

	// Ref support
	RefName      string // Ref name for object type
	ItemRefName  string // Ref name for array item type
	SchemaSkip   bool   // Whether to skip this field in Terraform schema generation
	IsDataSource bool   // Whether this field is part of a Data Source schema
	AttrTypeRef  string // Reference name for attribute type (helper function name)
	JsonTag      string // Custom JSON tag (optional)
	HasDefault   bool   // Whether field has a default value in OpenAPI schema
}

// ResourceData holds all data required to generate resource/sdk code
type ResourceData struct {
	Name                  string
	Service               string // e.g., "openstack", "marketplace"
	CleanName             string // e.g., "instance", "order"
	Plugin                string
	Operations            config.OperationSet
	APIPaths              map[string]string
	CreateFields          []FieldInfo
	UpdateFields          []FieldInfo
	ResponseFields        []FieldInfo
	ModelFields           []FieldInfo
	IsOrder               bool
	IsLink                bool
	IsDatasourceOnly      bool // True if this is a datasource-only definition (no resource)
	Source                *config.LinkResourceConfig
	Target                *config.LinkResourceConfig
	LinkCheckKey          string
	OfferingType          string
	UpdateActions         []UpdateAction
	StandaloneActions     []UpdateAction
	TerminationAttributes []config.ParameterConfig
	CreateOperation       *config.CreateOperationConfig
	CompositeKeys         []string
	NestedStructs         []FieldInfo // Only used for legacy resource generation if needed
	FilterParams          []FilterParam
	BaseOperationID       string // Base operation ID for actions
	HasDataSource         bool   // True if a corresponding data source exists
	SkipPolling           bool   // True if resource does not need polling (e.g. Structure Project)
}

// UpdateAction represents an enriched update action with resolved API path
type UpdateAction struct {
	Name       string // Action name (e.g., "update_limits")
	Operation  string // OpenAPI operation ID
	Param      string // Parameter name for payload
	CompareKey string // Field to compare for changes
	Path       string // Resolved API path from OpenAPI
}

// FilterParam describes a query parameter for filtering
type FilterParam struct {
	Name        string
	Type        string // String, Int64, Bool, Float64
	Description string
	Enum        []string // Allowed values for enum filters
}

// Clone creates a deep copy of FilterParam
func (p FilterParam) Clone() FilterParam {
	clone := p
	if p.Enum != nil {
		clone.Enum = make([]string, len(p.Enum))
		copy(clone.Enum, p.Enum)
	}
	return clone
}

// Clone creates a deep copy of FieldInfo
func (f FieldInfo) Clone() FieldInfo {
	clone := f
	if f.Enum != nil {
		clone.Enum = make([]string, len(f.Enum))
		copy(clone.Enum, f.Enum)
	}
	if f.ItemSchema != nil {
		clonedItem := f.ItemSchema.Clone()
		clone.ItemSchema = &clonedItem
	}
	if f.Properties != nil {
		clone.Properties = make([]FieldInfo, len(f.Properties))
		for i, prop := range f.Properties {
			clone.Properties[i] = prop.Clone()
		}
	}
	return clone
}

// GetGoType maps OpenAPI types to Terraform Plugin Framework types
func GetGoType(openAPIType string) string {
	switch openAPIType {
	case "string":
		return "types.String"
	case "integer":
		return "types.Int64"
	case "boolean":
		return "types.Bool"
	case "number":
		return "types.Float64"
	case "array":
		return "types.List"
	case "object":
		return "types.Object"
	default:
		return "types.String" // Fallback
	}
}

// GetFilterParamType maps OpenAPI/Go types to string identifiers used in FilterParam
func GetFilterParamType(goTypeStr string) string {
	switch goTypeStr {
	case "types.Int64":
		return "Int64"
	case "types.Bool":
		return "Bool"
	case "types.Float64":
		return "Float64"
	default:
		return "String"
	}
}
