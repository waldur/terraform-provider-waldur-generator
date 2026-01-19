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
