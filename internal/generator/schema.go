package generator

import (
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// FieldInfo represents information about a field extracted from OpenAPI schema
type FieldInfo struct {
	Name        string // JSON field name, e.g., "name"
	Type        string // OpenAPI type: "string", "integer", "boolean", "number"
	Required    bool   // Whether field is in schema.Required array
	ReadOnly    bool   // Whether field is marked readOnly in schema
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
	var propNames []string
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, propName := range propNames {
		propSchema := schema.Properties[propName]
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
			ReadOnly:    prop.ReadOnly,
			Description: prop.Description,
			TFSDKName:   propName,
			GoType:      mapTypeToGoType(typeStr),
		}

		fields = append(fields, field)
	}

	return fields, nil
}

// MergeFields combines two lists of fields, deduplicating by name.
// Fields from the first list take precedence for shared properties,
// but ReadOnly status is taken from either.
func MergeFields(primary, secondary []FieldInfo) []FieldInfo {
	fieldMap := make(map[string]FieldInfo)
	var merged []FieldInfo

	// Add primary fields first
	for _, f := range primary {
		fieldMap[f.Name] = f
		merged = append(merged, f)
	}

	// Add secondary fields if not present
	for _, f := range secondary {
		if existing, ok := fieldMap[f.Name]; ok {
			// Update existing field if secondary has more info (e.g. ReadOnly)
			if f.ReadOnly {
				existing.ReadOnly = true
				// Update in slice
				for i, mf := range merged {
					if mf.Name == existing.Name {
						merged[i] = existing
						break
					}
				}
			}
		} else {
			fieldMap[f.Name] = f
			merged = append(merged, f)
		}
	}

	return merged
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
