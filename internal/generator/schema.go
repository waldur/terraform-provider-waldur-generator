package generator

import (
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// FieldInfo represents information about a field extracted from OpenAPI schema
type FieldInfo struct {
	Name        string // JSON field name, e.g., "name"
	Type        string // OpenAPI type: "string", "integer", "boolean", "number", "array", "object"
	Required    bool   // Whether field is in schema.Required array
	ReadOnly    bool   // Whether field is marked readOnly in schema
	Description string // Field description from schema
	TFSDKName   string // Terraform SDK attribute name (same as Name for now)
	GoType      string // Terraform Framework type: "types.String", "types.List", "types.Object", etc.

	// Complex type support
	Enum       []string    // For enums: allowed values (only for string type)
	ItemType   string      // For arrays: type of items ("string", "integer", "object", etc.)
	ItemSchema *FieldInfo  // For arrays of objects: nested schema
	Properties []FieldInfo // For nested objects: object properties
}

// ExtractFields extracts field information from an OpenAPI schema reference
// Supports primitive types, enums, arrays (strings, objects), and nested objects
func ExtractFields(schemaRef *openapi3.SchemaRef) ([]FieldInfo, error) {
	return extractFieldsRecursive(schemaRef, 0, 3) // max depth: 3
}

// extractFieldsRecursive extracts field information with depth limiting
func extractFieldsRecursive(schemaRef *openapi3.SchemaRef, depth, maxDepth int) ([]FieldInfo, error) {
	if schemaRef == nil || schemaRef.Value == nil {
		return nil, nil
	}

	if depth > maxDepth {
		return nil, nil // Prevent infinite recursion
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
		// Skip uuid field as it's hard-coded in templates with tfsdk:"id"
		if propName == "uuid" {
			continue
		}

		propSchema := schema.Properties[propName]
		if propSchema == nil || propSchema.Value == nil {
			continue
		}

		prop := propSchema.Value
		typeStr := getSchemaType(prop)

		field := FieldInfo{
			Name:        propName,
			Type:        typeStr,
			Required:    requiredMap[propName],
			ReadOnly:    prop.ReadOnly,
			Description: prop.Description,
			TFSDKName:   propName,
		}

		// Handle different types
		switch typeStr {
		case "string":
			// Check for enum
			if len(prop.Enum) > 0 {
				field.Enum = make([]string, 0, len(prop.Enum))
				for _, e := range prop.Enum {
					if str, ok := e.(string); ok {
						field.Enum = append(field.Enum, str)
					}
				}
			}
			field.GoType = "types.String"
			fields = append(fields, field)

		case "integer":
			field.GoType = "types.Int64"
			fields = append(fields, field)

		case "boolean":
			field.GoType = "types.Bool"
			fields = append(fields, field)

		case "number":
			field.GoType = "types.Float64"
			fields = append(fields, field)

		case "array":
			// Extract array item type
			if prop.Items != nil && prop.Items.Value != nil {
				itemType := getSchemaType(prop.Items.Value)
				field.ItemType = itemType

				if itemType == "string" {
					field.GoType = "types.List"
					fields = append(fields, field)
				} else if itemType == "object" {
					// Array of objects - extract nested schema
					if nestedFields, err := extractFieldsRecursive(prop.Items, depth+1, maxDepth); err == nil && len(nestedFields) > 0 {
						// Store first nested field as representative schema
						if len(nestedFields) > 0 {
							field.ItemSchema = &nestedFields[0]
							field.ItemSchema.Properties = nestedFields
						}
						field.GoType = "types.List"
						fields = append(fields, field)
					}
				}
			}

		case "object":
			// Nested object - extract properties
			if nestedFields, err := extractFieldsRecursive(propSchema, depth+1, maxDepth); err == nil && len(nestedFields) > 0 {
				field.Properties = nestedFields
				field.GoType = "types.Object"
				fields = append(fields, field)
			}
		}
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

// MergeOrderFields combines offering (input) and resource (output) fields for Order resources.
// Input fields take precedence and determine writability.
// Output fields not in input are marked as ReadOnly (Computed).
func MergeOrderFields(input, output []FieldInfo) []FieldInfo {
	inputMap := make(map[string]bool)
	var merged []FieldInfo

	// Add input fields first
	for _, f := range input {
		// Special handling for project and offering
		if f.Name == "project" || f.Name == "offering" {
			f.Required = true
			f.ReadOnly = false
		}
		inputMap[f.Name] = true
		merged = append(merged, f)
	}

	// Ensure project and offering fields exist (required for Order resources)
	if !inputMap["project"] {
		merged = append(merged, FieldInfo{
			Name:        "project",
			Type:        "string",
			Required:    true,
			ReadOnly:    false,
			Description: "Project UUID",
			GoType:      "types.String",
			TFSDKName:   "project",
		})
		inputMap["project"] = true
	}
	if !inputMap["offering"] {
		merged = append(merged, FieldInfo{
			Name:        "offering",
			Type:        "string",
			Required:    true,
			ReadOnly:    false,
			Description: "Offering UUID or URL",
			GoType:      "types.String",
			TFSDKName:   "offering",
		})
		inputMap["offering"] = true
	}

	// Add output fields if not present in input
	for _, f := range output {
		if !inputMap[f.Name] {
			// Output-only fields are ReadOnly (Computed)
			f.ReadOnly = true
			f.Required = false
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
