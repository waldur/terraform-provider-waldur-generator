package common

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// ExtractFields extracts field information from an OpenAPI schema reference
// Supports primitive types, enums, arrays (strings, objects), and nested objects
func ExtractFields(cfg SchemaConfig, schemaRef *openapi3.SchemaRef, skipRootUUID bool) ([]FieldInfo, error) {
	return extractFieldsRecursive(cfg, schemaRef, "", 0, 3, skipRootUUID) // max depth: 3
}

// extractFieldsRecursive extracts field information with depth limiting
func extractFieldsRecursive(cfg SchemaConfig, schemaRef *openapi3.SchemaRef, pathPrefix string, depth, maxDepth int, skipRootUUID bool) ([]FieldInfo, error) {
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
		if depth == 0 && strings.ToLower(propName) == "uuid" && skipRootUUID {
			continue
		}

		fullPath := propName
		if pathPrefix != "" {
			fullPath = pathPrefix + "." + propName
		}

		if cfg.ExcludedFields[propName] || cfg.ExcludedFields[fullPath] {
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

		description := SanitizeString(prop.Description)
		if description == "" {
			description = Humanize(propName)
		}

		field := FieldInfo{
			Name:        propName,
			Type:        typeStr,
			Format:      prop.Format,
			Required:    requiredMap[propName],
			ReadOnly:    prop.ReadOnly,
			Description: description,
			RefName:     refName,
			Minimum:     prop.Min,
			Maximum:     prop.Max,
			Pattern:     prop.Pattern,
			HasDefault:  prop.Default != nil,
		}

		// Apply overrides
		if override, ok := cfg.FieldOverrides[fullPath]; ok {
			if override.Computed {
				field.ServerComputed = true
				field.UseStateForUnknown = true
			}
			field.UnknownIfNull = override.UnknownIfNull
			if override.Optional {
				field.Required = false
			}
			if override.Required {
				field.Required = true
			}
			if override.ForceNew {
				field.ForceNew = true
			}
		}

		// Handle different types
		field.GoType = GetGoType(typeStr)

		// Calculate SDK specific types
		CalculateSDKType(&field)

		switch typeStr {
		case OpenAPITypeString:
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

		case OpenAPITypeInteger, OpenAPITypeBoolean, OpenAPITypeNumber:
			fields = append(fields, field)

		case OpenAPITypeArray:
			// Extract array item type
			if prop.Items != nil && prop.Items.Value != nil {
				itemType := GetSchemaType(prop.Items.Value)
				field.ItemType = itemType

				// Extract item ref name
				if prop.Items.Ref != "" {
					parts := strings.Split(prop.Items.Ref, "/")
					field.ItemRefName = parts[len(parts)-1]
				}

				if itemType == OpenAPITypeString {
					if IsSetField(cfg, propName) {
						field.GoType = TFTypeSet
					} else {
						field.GoType = TFTypeList
					}
					// Recalculate SDK type after setting ItemType
					CalculateSDKType(&field)
					fields = append(fields, field)
				} else if itemType == OpenAPITypeObject {
					// Array of objects - extract nested schema
					if nestedFields, err := extractFieldsRecursive(cfg, prop.Items, fullPath, depth+1, maxDepth, false); err == nil && len(nestedFields) > 0 {
						// Store first nested field as representative schema
						if len(nestedFields) > 0 {
							field.ItemSchema = &FieldInfo{
								Type:       OpenAPITypeObject,
								GoType:     TFTypeObject,
								Properties: nestedFields,
								RefName:    field.ItemRefName, // Propagate ref name to item schema
							}
							CalculateSDKType(field.ItemSchema)
						}

						if IsSetField(cfg, propName) {
							field.GoType = TFTypeSet
						} else {
							field.GoType = TFTypeList
						}
						CalculateSDKType(&field)
						fields = append(fields, field)
					}
				} else {
					// Other primitive arrays (integer, etc)
					if IsSetField(cfg, propName) {
						field.GoType = TFTypeSet
					} else {
						field.GoType = TFTypeList
					}
					CalculateSDKType(&field)
					fields = append(fields, field)
				}
			}

		case OpenAPITypeObject:
			// Nested object - extract properties
			if nestedFields, err := extractFieldsRecursive(cfg, propSchema, fullPath, depth+1, maxDepth, false); err == nil && len(nestedFields) > 0 {
				field.Properties = nestedFields
				field.GoType = TFTypeObject
				CalculateSDKType(&field)
				fields = append(fields, field)
			} else if prop.AdditionalProperties.Schema != nil && prop.AdditionalProperties.Schema.Value != nil {
				// Handle maps with typed values (e.g., map[string]int)
				field.GoType = TFTypeMap
				itemType := GetSchemaType(prop.AdditionalProperties.Schema.Value)

				// Special case: 'prices' and 'switch_price' are defined as numbers but returned as strings
				// 'quotas' and 'marketplace_resource_count' are numbers and returned as numbers
				if itemType == OpenAPITypeNumber && (propName == "prices" || propName == "switch_price") {
					field.ItemType = OpenAPITypeString
				} else {
					field.ItemType = itemType
				}

				CalculateSDKType(&field)
				fields = append(fields, field)
			} else if err == nil {
				// Handle generic/dynamic objects (like attributes) as maps
				field.GoType = TFTypeMap
				field.ItemType = OpenAPITypeString // Default to Map[String]String
				CalculateSDKType(&field)
				fields = append(fields, field)
			}
		}
	}

	return fields, nil
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
				if goType == "" || strings.HasPrefix(goType, TFTypeList) || strings.HasPrefix(goType, TFTypeObject) {
					continue
				}

				var enumValues []string
				for _, val := range param.Schema.Value.Enum {
					enumValues = append(enumValues, fmt.Sprintf("%v", val))
				}

				filterParams = append(filterParams, FilterParam{
					Name:        param.Name,
					Type:        GetFilterParamType(goType),
					Description: param.Description,
					Enum:        enumValues,
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
	case OpenAPITypeString:
		return TFTypeString
	case OpenAPITypeInteger:
		return TFTypeInt64
	case OpenAPITypeBoolean:
		return TFTypeBool
	case OpenAPITypeNumber:
		return TFTypeFloat64
	case OpenAPITypeArray:
		return TFTypeList
	case OpenAPITypeObject:
		return TFTypeObject
	default:
		return TFTypeString // Fallback
	}
}

// GetFilterParamType maps OpenAPI/Go types to string identifiers used in FilterParam
func GetFilterParamType(goTypeStr string) string {
	return GoTypeToValidatorType(goTypeStr)
}
