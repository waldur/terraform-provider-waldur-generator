package common

import (
	"fmt"
	"sort"
	"strings"

	"github.com/waldur/terraform-provider-waldur-generator/internal/config"
)

// SchemaConfig defines field-level rules for schema extraction
type SchemaConfig struct {
	ExcludedFields map[string]bool
	SetFields      map[string]bool // Legacy global set fields
	FieldOverrides map[string]config.FieldConfig
}

// IsSetField checks if a field should be treated as a Set
func IsSetField(cfg SchemaConfig, name string) bool {
	if override, ok := cfg.FieldOverrides[name]; ok {
		return override.Set
	}
	return cfg.SetFields[name]
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
		if f.GoType == TFTypeObject {
			var subCreate, subResponse []FieldInfo
			if inCreate {
				subCreate = cf.Properties
			}
			if inResponse {
				subResponse = responseMap[f.Name].Properties
			}
			CalculateSchemaStatusRecursive(f.Properties, subCreate, subResponse)
		} else if (f.GoType == TFTypeList || f.GoType == TFTypeSet) && f.ItemSchema != nil {
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

// PrepareSetElementMatching replaces per-attribute UseStateForUnknown inside set
// elements with element-level matching. Inside a set the framework cannot find
// the prior element an attribute modifier belongs to, so it pairs elements by
// position and gives one element another's computed values.
func PrepareSetElementMatching(fields []FieldInfo) {
	for i := range fields {
		f := &fields[i]
		if f.GoType == TFTypeSet && f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0 {
			clearUseStateForUnknown(f.ItemSchema.Properties)
			if !f.ReadOnly {
				var keys []string
				for _, prop := range f.ItemSchema.Properties {
					if !prop.ReadOnly && !prop.SchemaSkip {
						keys = append(keys, prop.Name)
					}
				}
				sort.Strings(keys)
				f.SetMatchKeys = keys
			}
		}
		if len(f.Properties) > 0 {
			PrepareSetElementMatching(f.Properties)
		}
		if f.ItemSchema != nil && len(f.ItemSchema.Properties) > 0 {
			PrepareSetElementMatching(f.ItemSchema.Properties)
		}
	}
}

func clearUseStateForUnknown(fields []FieldInfo) {
	for i := range fields {
		fields[i].UseStateForUnknown = false
		clearUseStateForUnknown(fields[i].Properties)
		if fields[i].ItemSchema != nil {
			fields[i].ItemSchema.UseStateForUnknown = false
			clearUseStateForUnknown(fields[i].ItemSchema.Properties)
		}
	}
}
