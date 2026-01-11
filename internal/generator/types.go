package generator

import (
	"strings"
)

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

// ToTitle converts a string to title case for use in templates
// Exported so it can be used across the package if needed, though originally in generator.go
func ToTitle(s string) string {
	// Convert snake_case to TitleCase
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
