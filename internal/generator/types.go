package generator

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

// FilterParam describes a query parameter for filtering
type FilterParam struct {
	Name        string
	Type        string // String, Int64, Bool, Float64
	Description string
}

// UpdateAction represents an enriched update action with resolved API path
type UpdateAction struct {
	Name       string // Action name (e.g., "update_limits")
	Operation  string // OpenAPI operation ID
	Param      string // Parameter name for payload
	CompareKey string // Field to compare for changes
	Path       string // Resolved API path from OpenAPI
}
