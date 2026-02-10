package common

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaConfig defines field-level rules for schema extraction
type SchemaConfig struct {
	ExcludedFields map[string]bool
	SetFields      map[string]bool
}

// IsSetField checks if a field should be treated as a Set
func IsSetField(cfg SchemaConfig, name string) bool {
	return cfg.SetFields[name]
}

// ExtractFields extracts field information from an OpenAPI schema reference
// Supports primitive types, enums, arrays (strings, objects), and nested objects
func ExtractFields(cfg SchemaConfig, schemaRef *openapi3.SchemaRef, skipRootUUID bool) ([]FieldInfo, error) {
	return extractFieldsRecursive(cfg, schemaRef, 0, 3, skipRootUUID) // max depth: 3
}

// extractFieldsRecursive extracts field information with depth limiting
func extractFieldsRecursive(cfg SchemaConfig, schemaRef *openapi3.SchemaRef, depth, maxDepth int, skipRootUUID bool) ([]FieldInfo, error) {
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

		if cfg.ExcludedFields[propName] {
			continue
		}

		propSchema := schema.Properties[propName]
		if propSchema == nil || propSchema.Value == nil {
			continue
		}

		prop := propSchema.Value
		typeStr := GetSchemaType(prop)

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
				itemType := GetSchemaType(prop.Items.Value)
				field.ItemType = itemType

				// Extract item ref name
				if prop.Items.Ref != "" {
					parts := strings.Split(prop.Items.Ref, "/")
					field.ItemRefName = parts[len(parts)-1]
				}

				if itemType == "string" {
					if IsSetField(cfg, propName) {
						field.GoType = "types.Set"
					} else {
						field.GoType = "types.List"
					}
					fields = append(fields, field)
				} else if itemType == "object" {
					// Array of objects - extract nested schema
					if nestedFields, err := extractFieldsRecursive(cfg, prop.Items, depth+1, maxDepth, false); err == nil && len(nestedFields) > 0 {
						// Store first nested field as representative schema
						if len(nestedFields) > 0 {
							field.ItemSchema = &FieldInfo{
								Type:       "object",
								GoType:     "types.Object",
								Properties: nestedFields,
								RefName:    field.ItemRefName, // Propagate ref name to item schema
							}
						}

						if IsSetField(cfg, propName) {
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
			if nestedFields, err := extractFieldsRecursive(cfg, propSchema, depth+1, maxDepth, false); err == nil && len(nestedFields) > 0 {
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
			if len(existing.Properties) > 0 && len(f.Properties) > 0 {
				existing.Properties = MergeFields(existing.Properties, f.Properties)
			}
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

// GetSchemaType extracts the type string from openapi3.Schema
func GetSchemaType(schema *openapi3.Schema) string {
	if schema.Type != nil {
		// Types can be a slice, take the first one
		if len(*schema.Type) > 0 {
			return (*schema.Type)[0]
		}
	}

	// Fallback for OneOf/AnyOf/AllOf
	if len(schema.OneOf) > 0 {
		return GetSchemaType(schema.OneOf[0].Value)
	}
	if len(schema.AnyOf) > 0 {
		return GetSchemaType(schema.AnyOf[0].Value)
	}
	if len(schema.AllOf) > 0 {
		return GetSchemaType(schema.AllOf[0].Value)
	}

	return ""
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

// ApplySchemaSkipRecursive applies SchemaSkip to fields in cfg.ExcludedFields but not in inputFields.
func ApplySchemaSkipRecursive(cfg SchemaConfig, fields []FieldInfo, inputFields map[string]bool) {
	for i := range fields {
		f := &fields[i]
		if cfg.ExcludedFields[f.Name] && !inputFields[f.Name] {
			f.SchemaSkip = true
		}
		if len(f.Properties) > 0 {
			ApplySchemaSkipRecursive(cfg, f.Properties, inputFields)
		}
		if f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0 {
			ApplySchemaSkipRecursive(cfg, f.ItemSchema.Properties, inputFields)
		}
	}
}

// CalculateSchemaStatusRecursive recursively determines ServerComputed, UseStateForUnknown,
// and adjusts Required status for nested fields.
func CalculateSchemaStatusRecursive(fields []FieldInfo, createFields, responseFields []FieldInfo) {
	createMap := make(map[string]FieldInfo)
	for _, f := range createFields {
		createMap[f.Name] = f
	}

	responseMap := make(map[string]FieldInfo)
	for _, f := range responseFields {
		responseMap[f.Name] = f
	}

	for i := range fields {
		f := &fields[i]

		// ServerComputed logic
		cf, inCreate := createMap[f.Name]
		_, inResponse := responseMap[f.Name]

		if f.ReadOnly {
			f.ServerComputed = false
		} else if !inCreate {
			f.ServerComputed = true
		} else if !cf.Required && inResponse {
			f.ServerComputed = true
		}

		// UseStateForUnknown logic
		if f.ServerComputed || f.ReadOnly {
			f.UseStateForUnknown = true
		}

		// If it's ServerComputed, it shouldn't be Required in Terraform
		if f.ServerComputed && f.Required {
			f.Required = false
		}

		// Recursively process nested types
		if f.GoType == "types.Object" {
			var subCreate, subResponse []FieldInfo
			if inCreate {
				subCreate = cf.Properties
			}
			if inResponse {
				subResponse = responseMap[f.Name].Properties
			}
			CalculateSchemaStatusRecursive(f.Properties, subCreate, subResponse)
		} else if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemSchema != nil {
			var subCreate, subResponse []FieldInfo
			if inCreate && cf.ItemSchema != nil {
				subCreate = cf.ItemSchema.Properties
			}
			if inResponse && responseMap[f.Name].ItemSchema != nil {
				subResponse = responseMap[f.Name].ItemSchema.Properties
			}
			CalculateSchemaStatusRecursive(f.ItemSchema.Properties, subCreate, subResponse)
		}
	}
}

// CollectUniqueStructs gathers all Nested structs that have a AttrTypeRef defined
func CollectUniqueStructs(params ...[]FieldInfo) []FieldInfo {
	seen := make(map[string]bool)
	var result []FieldInfo
	var traverse func([]FieldInfo)

	traverse = func(fields []FieldInfo) {
		for _, f := range fields {
			// Check object type with AttrTypeRef or RefName
			if f.GoType == "types.Object" {
				key := f.AttrTypeRef
				if key == "" {
					key = f.RefName
				}
				if key != "" {
					if !seen[key] {
						seen[key] = true
						// Ensure AttrTypeRef is set for consistency in result
						if f.AttrTypeRef == "" {
							f.AttrTypeRef = key
						}
						result = append(result, f)
						traverse(f.Properties)
					}
				} else {
					traverse(f.Properties)
				}
			}
			// Check list/set of objects with AttrTypeRef or RefName
			if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemSchema != nil {
				key := f.ItemSchema.AttrTypeRef
				if key == "" {
					key = f.ItemSchema.RefName
				}
				if key != "" {
					if !seen[key] {
						seen[key] = true
						// Ensure AttrTypeRef is set
						if f.ItemSchema.AttrTypeRef == "" {
							f.ItemSchema.AttrTypeRef = key
						}
						result = append(result, *f.ItemSchema)
						traverse(f.ItemSchema.Properties)
					}
				} else {
					traverse(f.ItemSchema.Properties)
				}
			}
		}
	}

	for _, p := range params {
		traverse(p)
	}

	sort.Slice(result, func(i, j int) bool { return result[i].AttrTypeRef < result[j].AttrTypeRef })
	return result
}

// AssignMissingAttrTypeRefs recursively assigns a AttrTypeRef to objects/lists of objects that lack one.
func AssignMissingAttrTypeRefs(cfg SchemaConfig, fields []FieldInfo, prefix string, seenHashes map[string]string, seenNames map[string]string) {
	for i := range fields {
		f := &fields[i]

		// Recursively process children first (Bottom-Up)
		if f.GoType == "types.Object" {
			AssignMissingAttrTypeRefs(cfg, f.Properties, prefix+toTitle(f.Name), seenHashes, seenNames)
		} else if (f.GoType == "types.List" || f.GoType == "types.Set") && f.ItemSchema != nil {
			if f.ItemSchema.GoType == "types.Object" {
				AssignMissingAttrTypeRefs(cfg, f.ItemSchema.Properties, prefix+toTitle(f.Name), seenHashes, seenNames)

				// Also assign ref to ItemSchema itself
				hash := computeStructHash(*f.ItemSchema)
				if name, ok := seenHashes[hash]; ok {
					f.ItemSchema.AttrTypeRef = name
				} else {
					candidate := f.ItemSchema.RefName
					if candidate == "" {
						candidate = prefix + toTitle(f.Name)
					}
					finalName := resolveUniqueName(candidate, hash, seenNames)
					seenHashes[hash] = finalName
					seenNames[finalName] = hash
					f.ItemSchema.AttrTypeRef = finalName
				}
			}
		}

		// Now process f itself if it is Object
		if f.GoType == "types.Object" {
			hash := computeStructHash(*f)
			if name, ok := seenHashes[hash]; ok {
				f.AttrTypeRef = name
			} else {
				candidate := f.RefName
				if candidate == "" {
					candidate = prefix + toTitle(f.Name)
				}
				finalName := resolveUniqueName(candidate, hash, seenNames)
				seenHashes[hash] = finalName
				seenNames[finalName] = hash
				f.AttrTypeRef = finalName
			}
		}
	}
}

func resolveUniqueName(candidate string, hash string, seenNames map[string]string) string {
	finalName := candidate
	counter := 2
	for {
		if oldHash, exists := seenNames[finalName]; exists {
			if oldHash == hash {
				return finalName
			}
			finalName = fmt.Sprintf("%s%d", candidate, counter)
			counter++
		} else {
			return finalName
		}
	}
}

func computeStructHash(f FieldInfo) string {
	var parts []string
	for _, p := range f.Properties {
		key := fmt.Sprintf("%s:%s:%s", p.Name, p.GoType, p.AttrTypeRef)
		parts = append(parts, key)
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func toTitle(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// ExtractFilterParams extracts filter parameters from an OpenAPI operation
func ExtractFilterParams(op *openapi3.Operation, resourceName string) []FilterParam {
	var filterParams []FilterParam
	if op == nil {
		return filterParams
	}

	for _, paramRef := range op.Parameters {
		if paramRef.Value == nil {
			continue
		}
		param := paramRef.Value
		if param.In == "query" {
			paramName := param.Name
			if paramName == "page" || paramName == "page_size" || paramName == "o" || paramName == "field" {
				continue
			}
			if param.Schema != nil && param.Schema.Value != nil {
				typeStr := GetSchemaType(param.Schema.Value)
				goType := GetGoType(typeStr)
				if goType == "" || strings.HasPrefix(goType, "types.List") || strings.HasPrefix(goType, "types.Object") {
					continue
				}

				filterParams = append(filterParams, FilterParam{
					Name:        param.Name,
					Type:        GetFilterParamType(goType),
					Description: param.Description,
				})
			}
		}
	}
	sort.Slice(filterParams, func(i, j int) bool { return filterParams[i].Name < filterParams[j].Name })

	if resourceName != "" {
		for i := range filterParams {
			filterParams[i].Description = GetDefaultDescription(filterParams[i].Name, resourceName, filterParams[i].Description)
		}
	}

	return filterParams
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
