package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// FieldInfo represents information about a field extracted from OpenAPI schema
type FieldInfo struct {
	Name           string // JSON field name, e.g., "name"
	Type           string // OpenAPI type: "string", "integer", "boolean", "number", "array", "object"
	Required       bool   // Whether field is in schema.Required array
	ReadOnly       bool   // Whether field is marked readOnly in schema
	Description    string // Field description from schema
	Format         string // OpenAPI format: "date-time", "uuid", etc.
	GoType         string // Terraform Framework type: "types.String", "types.List", "types.Object", etc.
	ForceNew       bool   // Whether field requires replacement on change (immutable)
	ServerComputed bool   // Whether value can be set by server (readOnly or response-only)
	IsPathParam    bool   // Whether field is a path parameter (should not be in JSON body)

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

// ExtractFields extracts field information from an OpenAPI schema reference
// Supports primitive types, enums, arrays (strings, objects), and nested objects
func ExtractFields(schemaRef *openapi3.SchemaRef, skipRootUUID bool) ([]FieldInfo, error) {
	return extractFieldsRecursive(schemaRef, 0, 3, skipRootUUID) // max depth: 3
}

// extractFieldsRecursive extracts field information with depth limiting
func extractFieldsRecursive(schemaRef *openapi3.SchemaRef, depth, maxDepth int, skipRootUUID bool) ([]FieldInfo, error) {
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

	// Flatten allOf if present
	if len(schema.AllOf) > 0 {
		for _, subSchemaRef := range schema.AllOf {
			if subSchemaRef.Value == nil {
				continue
			}
			// Merge properties from allOf schema
			for name, prop := range subSchemaRef.Value.Properties {
				if schema.Properties == nil {
					schema.Properties = make(map[string]*openapi3.SchemaRef)
				}
				if _, exists := schema.Properties[name]; !exists {
					schema.Properties[name] = prop
				}
			}
			// Merge required fields
			for _, req := range subSchemaRef.Value.Required {
				requiredMap[req] = true
			}
		}
	}

	// Extract fields from properties
	var propNames []string
	for name := range schema.Properties {
		propNames = append(propNames, name)
	}
	sort.Strings(propNames)

	for _, propName := range propNames {
		// Skip uuid field if requested (hard-coded in templates with tfsdk:"id")
		if depth == 0 && propName == "uuid" && skipRootUUID {
			continue
		}

		if ExcludedFields[propName] {
			continue
		}

		propSchema := schema.Properties[propName]
		if propSchema == nil || propSchema.Value == nil {
			continue
		}

		prop := propSchema.Value
		typeStr := getSchemaType(prop)

		// Override incorrect schema types for billing fields
		if (propName == "total" || propName == "tax" || propName == "tax_current" || propName == "current") && typeStr == "string" {
			typeStr = "number"
			prop.Pattern = "" // Clear string-only pattern
		}

		refName := ""
		if propSchema.Ref != "" {
			parts := strings.Split(propSchema.Ref, "/")
			refName = parts[len(parts)-1]
		}

		field := FieldInfo{
			Name:        propName,
			Type:        typeStr,
			Format:      prop.Format,
			Required:    requiredMap[propName],
			ReadOnly:    prop.ReadOnly,
			Description: SanitizeString(prop.Description),
			RefName:     refName,
			Minimum:     prop.Min,
			Maximum:     prop.Max,
			Pattern:     prop.Pattern,
			HasDefault:  prop.Default != nil,
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

				// Extract item ref name
				if prop.Items.Ref != "" {
					parts := strings.Split(prop.Items.Ref, "/")
					field.ItemRefName = parts[len(parts)-1]
				}

				if itemType == "string" {
					if IsSetField(propName) {
						field.GoType = "types.Set"
					} else {
						field.GoType = "types.List"
					}
					fields = append(fields, field)
				} else if itemType == "object" {
					// Array of objects - extract nested schema
					if nestedFields, err := extractFieldsRecursive(prop.Items, depth+1, maxDepth, false); err == nil && len(nestedFields) > 0 {
						// Store first nested field as representative schema
						if len(nestedFields) > 0 {
							field.ItemSchema = &FieldInfo{
								Type:       "object",
								GoType:     "types.Object",
								Properties: nestedFields,
								RefName:    field.ItemRefName, // Propagate ref name to item schema
							}
						}

						if IsSetField(propName) {
							field.GoType = "types.Set"
						} else {
							field.GoType = "types.List"
						}
						fields = append(fields, field)
					}
				}
			}

		case "object":
			// Nested object - extract properties
			// Pass the RefName to the recursive call? No, ExtractFields works on schema.
			if nestedFields, err := extractFieldsRecursive(propSchema, depth+1, maxDepth, false); err == nil && len(nestedFields) > 0 {
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
			// Preserve IsPathParam from primary - path params should keep their Required state
			if existing.IsPathParam {
				existing.ReadOnly = false // Path params are always writable
			} else if f.ReadOnly {
				existing.ReadOnly = true
			}
			if f.ServerComputed {
				existing.ServerComputed = true
			}

			// Recursively merge nested properties if present in both
			// Case 1: Nested objects (Properties)
			if len(existing.Properties) > 0 && len(f.Properties) > 0 {
				existing.Properties = MergeFields(existing.Properties, f.Properties)
			}
			// Case 2: Array of objects (ItemSchema.Properties)
			if existing.ItemSchema != nil && f.ItemSchema != nil && len(existing.ItemSchema.Properties) > 0 && len(f.ItemSchema.Properties) > 0 {
				existing.ItemSchema.Properties = MergeFields(existing.ItemSchema.Properties, f.ItemSchema.Properties)
			}

			// Update in slice
			for i, mf := range merged {
				if mf.Name == existing.Name {
					merged[i] = existing
					break
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

			// If it appears in both input and output, it's server-computed
			existing.ServerComputed = true
			updated = true

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
	// Marketplace fields
	"marketplace_category_name":  true,
	"marketplace_category_uuid":  true,
	"marketplace_offering_name":  true,
	"marketplace_offering_uuid":  true,
	"marketplace_plan_uuid":      true,
	"marketplace_resource_state": true,
	"is_limit_based":             true,
	"is_usage_based":             true,
	"access_url":                 true,
	// Service settings
	"service_name":                   true,
	"service_settings":               true,
	"service_settings_error_message": true,
	"service_settings_state":         true,
	"service_settings_uuid":          true,
	// Project/Customer (handled separately)
	"project_name":          true,
	"project_uuid":          true,
	"customer_abbreviation": true,
	"customer_name":         true,
	"customer_native_name":  true,
	"customer_uuid":         true,
}

// SetFields defines fields that should be treated as Sets instead of Lists
// This is used for unordered collections to avoid permadiffs in Terraform
var SetFields = map[string]bool{
	"security_groups": true,
	"floating_ips":    true,
	"tags":            true,
	"ssh_keys":        true,
}

// IsSetField checks if a field should be treated as a Set
func IsSetField(name string) bool {
	return SetFields[name]
}

// GetDefaultDescription returns a generated description based on the field name if the current description is empty or too short.
// It always returns a sanitized string.
func GetDefaultDescription(name, resourceName, currentDesc string) string {
	desc := ""
	if len(strings.TrimSpace(currentDesc)) >= 2 {
		desc = currentDesc
	} else if strings.HasSuffix(name, "_uuid") {
		base := strings.TrimSuffix(name, "_uuid")
		desc = fmt.Sprintf("UUID of the %s", strings.ReplaceAll(base, "_", " "))
	} else if strings.HasSuffix(name, "_name") {
		base := strings.TrimSuffix(name, "_name")
		desc = fmt.Sprintf("Name of the %s", strings.ReplaceAll(base, "_", " "))
	} else if strings.HasSuffix(name, "_id") {
		base := strings.TrimSuffix(name, "_id")
		desc = fmt.Sprintf("ID of the %s", strings.ReplaceAll(base, "_", " "))
	} else if name == "name" {
		desc = fmt.Sprintf("Name of the %s", resourceName)
	} else if name == "description" {
		desc = fmt.Sprintf("Description of the %s", resourceName)
	} else if name == "uuid" {
		desc = fmt.Sprintf("UUID of the %s", resourceName)
	} else if strings.HasPrefix(name, "is_") {
		base := strings.TrimPrefix(name, "is_")
		desc = fmt.Sprintf("Is %s", strings.ReplaceAll(base, "_", " "))
	} else {
		// Fallback: sentence case from snake_case
		human := strings.ReplaceAll(name, "_", " ")
		if len(human) > 0 {
			desc = strings.ToUpper(human[:1]) + human[1:]
		} else {
			desc = " "
		}
	}

	return SanitizeString(desc)
}

// FillDescriptions recursively populates missing descriptions for fields.
// It uses a combination of direct mappings (e.g. uuid -> "UUID of the resource")
// and heuristics (trailing suffixes, is_ prefix, snake_case to Sentence case).
func FillDescriptions(fields []FieldInfo, resourceName string) {
	for i := range fields {
		f := &fields[i]
		f.Description = GetDefaultDescription(f.Name, resourceName, f.Description)

		// Recurse for nested properties
		if len(f.Properties) > 0 {
			FillDescriptions(f.Properties, resourceName)
		}
		if f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0 {
			FillDescriptions(f.ItemSchema.Properties, resourceName)
		}
	}
}

// ApplySchemaSkipRecursive applies SchemaSkip to fields in ExcludedFields but not in inputFields.
func ApplySchemaSkipRecursive(fields []FieldInfo, inputFields map[string]bool) {
	for i := range fields {
		f := &fields[i]
		if ExcludedFields[f.Name] && !inputFields[f.Name] {
			f.SchemaSkip = true
		}
		if len(f.Properties) > 0 {
			ApplySchemaSkipRecursive(f.Properties, inputFields)
		}
		if f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0 {
			ApplySchemaSkipRecursive(f.ItemSchema.Properties, inputFields)
		}
	}
}
