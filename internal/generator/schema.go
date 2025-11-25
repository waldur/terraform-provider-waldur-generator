package generator

import (
	"github.com/getkin/kin-openapi/openapi3"
)

// FieldInfo represents information about a field extracted from OpenAPI schema
type FieldInfo struct {
	Name        string // JSON field name, e.g., "name"
	Type        string // OpenAPI type: "string", "integer", "boolean", "number"
	Required    bool   // Whether field is in schema.Required array
	Description string // Field description from schema
	TFSDKName   string // Terraform SDK attribute name (same as Name for now)
	GoType      string // Terraform Framework type: "types.String", "types.Int64", "types.Bool"
}

// ExtractFields extracts field information from an OpenAPI schema reference
// Currently supports primitive types only: string, integer, boolean, number
func ExtractFields(schemaRef *openapi3.SchemaRef) ([]FieldInfo, error) {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil, nil
	}

	schema := schemaRef.Value
	var fields []FieldInfo

	// Build a map of required fields for quick lookup
	requiredMap := make(map[string]bool)
	for _, req := range schema.Required {
		requiredMap[req] = true
	}

	// Extract fields from properties
	for propName, propSchema := range schema.Properties {
		if propSchema == nil || propSchema.Value == nil {
			continue
		}

		prop := propSchema.Value

		// Get the type - openapi3 uses Types which can have multiple types
		typeStr := getSchemaType(prop)

		// Only handle primitive types for now
		if !isPrimitiveType(typeStr) {
			continue
		}

		field := FieldInfo{
			Name:        propName,
			Type:        typeStr,
			Required:    requiredMap[propName],
			Description: prop.Description,
			TFSDKName:   propName,
			GoType:      mapTypeToGoType(typeStr),
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// getSchemaType extracts the type string from openapi3.Schema
func getSchemaType(schema *openapi3.Schema) string {
	if schema.Type == nil {
		return ""
	}
	// Types can be a slice, take the first one
	if len(*schema.Type) > 0 {
		return (*schema.Type)[0]
	}
	return ""
}

// isPrimitiveType checks if the OpenAPI type is a supported primitive
func isPrimitiveType(typeVal string) bool {
	switch typeVal {
	case "string", "integer", "boolean", "number":
		return true
	default:
		return false
	}
}

// mapTypeToGoType maps OpenAPI type to Terraform Framework Go type
func mapTypeToGoType(openAPIType string) string {
	switch openAPIType {
	case "string":
		return "types.String"
	case "integer":
		return "types.Int64"
	case "boolean":
		return "types.Bool"
	case "number":
		return "types.Float64"
	default:
		return "types.String" // fallback
	}
}
