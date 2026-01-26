package generator

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// toTitle converts a string to title case for use in templates
func toTitle(s string) string {
	// Convert snake_case to TitleCase
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// humanize converts snake_case to Title Case with spaces
func humanize(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// displayName strips module prefix (anything before first underscore) and converts to title case for user-facing messages
func displayName(s string) string {
	// Strip everything before first underscore (e.g., "structure_project" -> "project")
	name := s
	if idx := strings.Index(s, "_"); idx != -1 {
		name = s[idx+1:]
	}

	// Convert to title case
	return toTitle(name)
}

// ToAttrType converts FieldInfo to proper attr.Type expression used in Terraform schema
func ToAttrType(f FieldInfo) string {
	switch f.GoType {
	case "types.String":
		return "types.StringType"
	case "types.Int64":
		return "types.Int64Type"
	case "types.Bool":
		return "types.BoolType"
	case "types.Float64":
		return "types.Float64Type"
	case "types.List":
		if f.ItemType == "object" && f.ItemSchema != nil {
			var attrTypes []string
			// Sort properties for deterministic output
			sortedProps := f.ItemSchema.Properties
			// We can't easily sort here without mutating or copying, relying on ExtractFields sort
			for _, prop := range sortedProps {
				attrTypes = append(attrTypes, "\""+prop.Name+"\": "+ToAttrType(prop))
			}
			content := strings.Join(attrTypes, ",\n")
			if len(attrTypes) > 0 {
				content += ","
			}
			objType := "types.ObjectType{AttrTypes: map[string]attr.Type{\n" + content + "\n}}"
			return "types.ListType{ElemType: " + objType + "}"
		}
		// List of primitives
		elemType := "types.StringType"
		if f.ItemType == "integer" {
			elemType = "types.Int64Type"
		} else if f.ItemType == "boolean" {
			elemType = "types.BoolType"
		} else if f.ItemType == "number" {
			elemType = "types.Float64Type"
		}
		return "types.ListType{ElemType: " + elemType + "}"
	case "types.Set":
		if f.ItemType == "object" && f.ItemSchema != nil {
			var attrTypes []string
			// Sort properties for deterministic output
			sortedProps := f.ItemSchema.Properties
			// We can't easily sort here without mutating or copying, relying on ExtractFields sort
			for _, prop := range sortedProps {
				attrTypes = append(attrTypes, "\""+prop.Name+"\": "+ToAttrType(prop))
			}
			content := strings.Join(attrTypes, ",\n")
			if len(attrTypes) > 0 {
				content += ","
			}
			objType := "types.ObjectType{AttrTypes: map[string]attr.Type{\n" + content + "\n}}"
			return "types.SetType{ElemType: " + objType + "}"
		}
		// Set of primitives
		elemType := "types.StringType"
		if f.ItemType == "integer" {
			elemType = "types.Int64Type"
		} else if f.ItemType == "boolean" {
			elemType = "types.BoolType"
		} else if f.ItemType == "number" {
			elemType = "types.Float64Type"
		}
		return "types.SetType{ElemType: " + elemType + "}"
	case "types.Object":
		var attrTypes []string
		for _, prop := range f.Properties {
			attrTypes = append(attrTypes, "\""+prop.Name+"\": "+ToAttrType(prop))
		}
		content := strings.Join(attrTypes, ",\n")
		if len(attrTypes) > 0 {
			content += ","
		}
		return "types.ObjectType{AttrTypes: map[string]attr.Type{\n" + content + "\n}}"
	default:
		return "types.StringType"
	}
}

// formatValidatorValue formats a float64 for use in validators, handling integer truncation and large values
func formatValidatorValue(v float64, goType string) string {
	if goType == "types.Int64" {
		// If the value is very large (likely representing MaxInt64 but lost precision in float64),
		// it's safer to skip the validator to avoid overflow errors in generated code.
		if v > 9e18 || v < -9e18 {
			return ""
		}
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%g", v)
}

// GetFuncMap returns the common template functions
func GetFuncMap() template.FuncMap {
	return template.FuncMap{
		"title":             toTitle,
		"humanize":          humanize,
		"displayName":       displayName,
		"toAttrType":        ToAttrType,
		"toFilterParamType": GetFilterParamType,
		"formatValidator":   formatValidatorValue,
		"sanitize": func(s string) string {
			// Replace problematic characters in descriptions
			s = strings.ReplaceAll(s, "\\", "\\\\") // Escape backslashes first
			s = strings.ReplaceAll(s, "\"", "\\\"") // Escape quotes
			s = strings.ReplaceAll(s, "\n", " ")    // Replace newlines with spaces
			s = strings.ReplaceAll(s, "\r", "")     // Remove carriage returns
			s = strings.ReplaceAll(s, "\t", " ")    // Replace tabs with spaces
			// Normalize multiple spaces
			for strings.Contains(s, "  ") {
				s = strings.ReplaceAll(s, "  ", " ")
			}
			return strings.TrimSpace(s)
		},
		"replace": func(old, new, s string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"contains": strings.Contains,
		"isPathParam": func(op *config.CreateOperationConfig, fieldName string) bool {
			if op == nil {
				return false
			}
			for _, val := range op.PathParams {
				if val == fieldName {
					return true
				}
			}
			return false
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, nil
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, nil
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"makeSlice": func(items ...interface{}) interface{} {
			return items
		},
	}
}
