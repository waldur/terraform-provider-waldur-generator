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
	ForceNew    bool   // Whether field requires replacement on change (immutable)

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
		field.GoType = GetGoType(typeStr)

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
			fields = append(fields, field)

		case "integer", "boolean", "number":
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
							field.ItemSchema = &FieldInfo{
								Type:       "object",
								GoType:     "types.Object",
								Properties: nestedFields,
							}
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
// Nested objects/lists are merged recursively.
func MergeOrderFields(input, output []FieldInfo) []FieldInfo {
	merged := mergeOrderedFieldsRecursive(input, output)

	// Map for quick lookup
	fieldMap := make(map[string]int)
	for i, f := range merged {
		fieldMap[f.Name] = i
	}

	// Ensure project field exists and is configured correctly
	if idx, ok := fieldMap["project"]; ok {
		merged[idx].Required = true
		merged[idx].ReadOnly = false
	} else {
		merged = append(merged, FieldInfo{
			Name:        "project",
			Type:        "string",
			Required:    true,
			ReadOnly:    false,
			Description: "Project URL",
			GoType:      "types.String",
			TFSDKName:   "project",
		})
		fieldMap["project"] = len(merged) - 1
	}

	// Ensure offering field exists and is configured correctly
	if idx, ok := fieldMap["offering"]; ok {
		merged[idx].Required = true
		merged[idx].ReadOnly = false
	} else {
		merged = append(merged, FieldInfo{
			Name:        "offering",
			Type:        "string",
			Required:    true,
			ReadOnly:    false,
			Description: "Offering URL",
			GoType:      "types.String",
			TFSDKName:   "offering",
		})
		fieldMap["offering"] = len(merged) - 1
	}

	return merged
}

func mergeOrderedFieldsRecursive(input, output []FieldInfo) []FieldInfo {
	inputMap := make(map[string]bool)
	fieldIdx := make(map[string]int)
	var merged []FieldInfo

	// Add input fields first
	for _, f := range input {
		inputMap[f.Name] = true
		merged = append(merged, f)
		fieldIdx[f.Name] = len(merged) - 1
	}

	// Add output fields if not present in input, or merge if present
	for _, f := range output {
		if !inputMap[f.Name] {
			// Output-only fields are ReadOnly (Computed)
			f.ReadOnly = true
			f.Required = false
			merged = append(merged, f)
		} else {
			// Merge nested properties
			idx := fieldIdx[f.Name]
			existing := merged[idx]
			updated := false

			// Merge nested lists of objects
			if existing.ItemType == "object" && f.ItemType == "object" && existing.ItemSchema != nil && f.ItemSchema != nil {
				existing.ItemSchema.Properties = mergeOrderedFieldsRecursive(existing.ItemSchema.Properties, f.ItemSchema.Properties)
				updated = true
			} else if existing.GoType == "types.Object" && f.GoType == "types.Object" {
				// Merge nested objects
				existing.Properties = mergeOrderedFieldsRecursive(existing.Properties, f.Properties)
				updated = true
			}

			if updated {
				merged[idx] = existing
			}
		}
	}

	return merged
}

// getSchemaType extracts the type string from openapi3.Schema
func getSchemaType(schema *openapi3.Schema) string {
	if schema.Type != nil {
		// Types can be a slice, take the first one
		if len(*schema.Type) > 0 {
			return (*schema.Type)[0]
		}
	}

	// Fallback for OneOf/AnyOf/AllOf
	if len(schema.OneOf) > 0 {
		return getSchemaType(schema.OneOf[0].Value)
	}
	if len(schema.AnyOf) > 0 {
		return getSchemaType(schema.AnyOf[0].Value)
	}
	if len(schema.AllOf) > 0 {
		return getSchemaType(schema.AllOf[0].Value)
	}

	return ""
}

// ExcludedFields defines fields that should be excluded from standard resources
var ExcludedFields = map[string]bool{
	"marketplace_category_name":      true,
	"marketplace_category_uuid":      true,
	"marketplace_offering_name":      true,
	"marketplace_offering_uuid":      true,
	"marketplace_plan_uuid":          true,
	"marketplace_resource_state":     true,
	"marketplace_resource_uuid":      true,
	"is_limit_based":                 true,
	"is_usage_based":                 true,
	"service_name":                   true,
	"service_settings":               true,
	"service_settings_error_message": true,
	"service_settings_state":         true,
	"service_settings_uuid":          true,
	"project":                        true,
	"project_name":                   true,
	"project_uuid":                   true,
	"customer":                       true,
	"customer_abbreviation":          true,
	"customer_name":                  true,
	"customer_native_name":           true,
	"customer_uuid":                  true,
}
