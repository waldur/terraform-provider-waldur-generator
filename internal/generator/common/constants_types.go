package common

// OpenAPI Types
const (
	OpenAPITypeString  = "string"
	OpenAPITypeInteger = "integer"
	OpenAPITypeBoolean = "boolean"
	OpenAPITypeNumber  = "number"
	OpenAPITypeArray   = "array"
	OpenAPITypeObject  = "object"
)

// Go SDK Types
const (
	GoTypeString  = "string"
	GoTypeInt64   = "int64"
	GoTypeFloat64 = "float64"
	GoTypeBool    = "bool"
	GoTypeMap     = "map[string]"
	GoTypeAny     = "interface{}"
)

// Terraform Plugin Framework Types
const (
	TFTypeString  = "types.String"
	TFTypeInt64   = "types.Int64"
	TFTypeFloat64 = "types.Float64"
	TFTypeBool    = "types.Bool"
	TFTypeList    = "types.List"
	TFTypeSet     = "types.Set"
	TFTypeMap     = "types.Map"
	TFTypeObject  = "types.Object"
)
